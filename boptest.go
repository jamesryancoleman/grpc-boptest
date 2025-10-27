package boptest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"maps"
)

var (
	Host = "0.0.0.0"

	termLogLevel = new(slog.LevelVar)
	fileLogLevel = new(slog.LevelVar)

	TermLog *slog.Logger
	FileLog *slog.Logger

	schemaRe = regexp.MustCompile(`^boptest://(?P<testCase>[a-zA-Z0-9\_\-.]*)/(?P<point>[a-zA-Z0-9\_\-.]+)$`)
)

const (
	HTTPStatus_Ok         = "200 OK"
	HTTPStatus_BadRequest = "400 Bad Request"

	ContentType_ApplicationJSON = "application/json"

	DefaultStep       = 3600 // 1 hour, per BOPTEST
	DefaultUpdateFreq = 1    // seecond
)

// called when the package is imported
func init() {
	// You can set the logging level programmatically, from a config file, or env variable.
	termLogLevel.Set(slog.LevelDebug)
	fileLogLevel.Set(slog.LevelDebug)

	// standard terminal logger
	TermLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: termLogLevel, // this can be set programmatically
	}))

	// create a file, if it doesn't exist, and write json log there
	file, err := os.OpenFile("boptest_log.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		TermLog.Error("Failed to open log file", "error", err)
		os.Exit(1)
	}
	// assign the terminal logger
	FileLog = slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: fileLogLevel, // this can be set programmatically
	}))

	TermLog.Info("TZ confirmation", "TZ", time.Local)
}

// a concurrency safe map
type SafeMap struct {
	data map[string]any
	sync.RWMutex
}

func (m *SafeMap) Set(key string, value any) {
	m.Lock()
	defer m.Unlock()
	if m.data == nil {
		m.data = map[string]any{key: value}
	} else {
		m.data[key] = value
	}
	// TermLog.Debug("set complete", "data", m.data)
}

func (m *SafeMap) SetAll(values map[string]any) {
	m.Lock()
	defer m.Unlock()

	m.data = make(map[string]any, len(values))
	maps.Copy(m.data, values)
}

func (m *SafeMap) Clear() {
	m.Lock()
	defer m.Unlock()
	clear(m.data)
}

func (m *SafeMap) GetAll() map[string]any {
	m.RLock()
	defer m.RUnlock()
	return maps.Clone(m.data)
}

func (m *SafeMap) Flush() map[string]any {
	m.Lock()
	defer m.Unlock()
	defer clear(m.data)
	if m.data == nil {
		return map[string]any{}
	}
	return maps.Clone(m.data)
}

// a concurrency safe map for storing simulation state
type StateMap SafeMap

// overwrites the whole map
func (m *StateMap) SetAll(newState map[string]any) {
	m.Lock()
	defer m.Unlock()

	m.data = make(map[string]any, len(newState))
	maps.Copy(m.data, newState)
}

func (m *StateMap) GetAll() map[string]any {
	m.RLock()
	defer m.RUnlock()

	copy := make(map[string]any, len(m.data))
	maps.Copy(copy, m.data)

	return copy
}

// returns int, float, bool, string, or nil, if not present
func (m *StateMap) Get(key string) any {
	m.RLock()
	defer m.RUnlock()
	if val, ok := m.data[key]; ok {
		return val
	}
	return nil
}

func (m *StateMap) GetMultiple(keys []string) map[string]any {
	m.RLock()
	defer m.RUnlock()

	results := make(map[string]any)
	for _, key := range keys {
		if val, ok := m.data[key]; ok {
			results[key] = val
		}
	}
	return results
}

// returns the current time of the simulation or an error
func (m *StateMap) Time() (time.Time, error) {
	m.RLock()
	defer m.RUnlock()

	val, ok := m.data["time"]
	if !ok {
		return time.Now(), fmt.Errorf("time not found in state map")
	}

	seconds, ok := val.(float64)
	if !ok {
		return time.Now(), fmt.Errorf("could not cast time as float")
	}

	// return time.Time using seconds
	currentYear := time.Date(time.Now().Year(), 1, 1, 0, 0, int(seconds), 0, time.Local)
	return currentYear, nil
}

type TestCase struct {
	ID   string `json:"testid"`
	Host string `json:"-"`

	stopCh chan struct{} `json:"-"`
	ticker *time.Ticker  `json:"-"`

	startNow bool `json:"-"`

	Created time.Time `json:"-"`
	Started time.Time `json:"-"`
	Stopped time.Time `json:"-"`

	step       int `json:"-"` // increment of time to advance the simulation by
	updateFreq int `json:"-"` // how often you want the simulation to recalculate

	StartTime int `json:"start_time"`    // seconds since start of year
	WarmUp    int `json:"warmup_period"` // seconds before startTime

	State StateMap `json:"-"`

	writeBuffer SafeMap `json:"-"`
}

type testCaseOption func(*TestCase)

// seconds since start of year
func WithHost(addr string) testCaseOption {
	return func(c *TestCase) {
		Host = addr
		c.Host = addr
	}
}

// seconds since start of year
func WithStartTime(t int) testCaseOption {
	return func(c *TestCase) {
		c.StartTime = t
	}
}

// seconds before startTime
func WithWarmUp(d int) testCaseOption {
	return func(c *TestCase) {
		c.WarmUp = d
	}
}

// seconds between steps
func WithStep(d int) testCaseOption {
	return func(c *TestCase) {
		c.step = d
	}
}

// seconds between steps
func WithUpdateFrequency(d int) testCaseOption {
	return func(c *TestCase) {
		c.updateFreq = d
	}
}

func WithStartNow() testCaseOption {
	return func(c *TestCase) {
		c.startNow = true
	}
}

type HTTPResponse struct {
	Status string
	Body   []byte
}

type JSONResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type PointsResponse struct {
	JSONResponse
	Payload map[string]PointProperties `json:"payload"`
}

type PointProperties struct {
	Unit        string
	Description string
	Minimum     float64
	Maximum     float64
}

type StateUpdate struct {
	JSONResponse
	State map[string]any `json:"payload"`
}

type SetStepResponse struct {
	JSONResponse
	Step int `json:"payload"`
}

type RunningResponse struct {
	JSONResponse
	Step int `json:"payload"`
}

type ErrorList struct {
	Errors []BoptestError
}

type BoptestError struct {
	Value    string `json:"value"`
	Msg      string `json:"msg"`
	Param    string `json:"param"`
	Location string `json:"location"`
}

func Get(url string) (HTTPResponse, error) {
	resp, err := http.Get(url)
	if err != nil {
		TermLog.Error(err.Error())
		FileLog.Error(err.Error())
		return HTTPResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		TermLog.Error(err.Error())
	}
	return HTTPResponse{
		Status: resp.Status,
		Body:   body,
	}, nil
}

func Put(url, contentType string, payload []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(payload))
	if err != nil {
		return []byte{}, err
	}
	req.Header.Set("Content-Type", contentType)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		TermLog.Error(err.Error())
	}
	return body, nil
}

func Post(url, contentType string, payload []byte) []byte {
	postBody := bytes.NewBuffer(payload)
	resp, err := http.Post(url, contentType, postBody)
	if err != nil {
		TermLog.Error(err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		TermLog.Error(err.Error())
	}
	return body
}

// takes the name of the testcase and returns the test id.
func NewTestCase(testcase string, opts ...testCaseOption) (*TestCase, error) {
	var c = &TestCase{}
	// initialize fields
	c.Host = Host // holds a default value
	c.State = StateMap{
		data: make(map[string]any),
	}
	c.step = DefaultStep
	c.updateFreq = DefaultUpdateFreq

	c.stopCh = make(chan struct{})
	c.Created = time.Now()

	// apply optional parameters
	// will override step and updateFreq
	for _, opt := range opts {
		opt(c)
	}

	url := fmt.Sprintf("http://%s/testcases/%s/select", c.Host, testcase)

	postBody := bytes.NewBuffer([]byte{})
	resp, err := http.Post(url, "text/raw", postBody)
	if err != nil {
		TermLog.Error(err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	fmt.Println(string(body))
	if err != nil {
		TermLog.Error(err.Error())
		return nil, err
	}

	err = json.Unmarshal(body, &c)
	if err != nil {
		TermLog.Error(err.Error())
		return c, err
	}

	FileLog.Info("created test case", "id", c.ID, "time", c.Created.String())

	// set the step if its not the default
	if c.step != DefaultStep {
		// because advance moves the simluation forward at the rate of c.step
		_step := int(math.Round(float64(c.step) * float64(c.updateFreq)))
		err := c.SetStep(_step)
		if err != nil {
			FileLog.Error("unable to set step", "test_case", c.ID)
			return c, err
		}
	}

	c.ticker = time.NewTicker(time.Duration(c.updateFreq * int(time.Second)))
	if !c.startNow {
		c.ticker.Stop()
	}

	go c.run()

	return c, nil
}

func StopTestCase(testId string) error {
	url := fmt.Sprintf("http://%s/stop/%s", Host, testId)
	_, err := Put(url, "", []byte{})
	if err != nil {
		return err
	}
	// fmt.Println(string(resp))
	return nil
}

func (c *TestCase) stop() error {
	if c.ticker != nil {
		c.ticker.Stop()
	}
	err := StopTestCase(c.ID)
	if err != nil {
		return err
	}
	c.Stopped = time.Now()
	FileLog.Info("stopped test case", "id", c.ID, "time", c.Stopped.String())
	return nil
}

func (c *TestCase) Stop() {
	// TermLog.Info("s†op called")
	c.stopCh <- struct{}{}
	// TermLog.Info("first send on stop chan")
	<-c.stopCh // unblocked by the run loop
}

// takes the testid and returns all possible measurements
func measurements(testid string) (map[string]PointProperties, error) {
	url := fmt.Sprintf("http://%s/measurements/%s", Host, testid)
	// fmt.Println(url)

	resp, err := http.Get(url)
	if err != nil {
		TermLog.Error(err.Error())
	}
	defer resp.Body.Close()

	if resp.Status == HTTPStatus_BadRequest {
		fmt.Printf("'%s'\n", resp.Status) // turn this to a log
		return nil, fmt.Errorf("%s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var r PointsResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}

	return r.Payload, nil
}

func (c *TestCase) Measurements() (map[string]PointProperties, error) {
	return measurements(c.ID)
}

func inputs(testid string) (map[string]PointProperties, error) {
	url := fmt.Sprintf("http://%s/inputs/%s", Host, testid)
	// fmt.Println(url)

	resp, err := Get(url)
	if err != nil {
		return nil, err
	}

	var r PointsResponse
	if err := json.Unmarshal(resp.Body, &r); err != nil {
		panic(err)
	}
	return r.Payload, nil
}

func (c *TestCase) Inputs() (map[string]PointProperties, error) {
	return inputs(c.ID)
}

// the TestCase gets a ticker assigned and the that activates the run loop
// that was created with NewTestCase().
func (c *TestCase) Start() error {
	// define t=0 and start simulation
	payload, err := json.Marshal(c)
	if err != nil {
		FileLog.Error(err.Error())
		return err
	}

	url := fmt.Sprintf("http://%s/initialize/%s", Host, c.ID)
	b, err := Put(url, "application/json", payload)
	if err != nil {
		FileLog.Error(err.Error())
		return err
	}

	var resp StateUpdate
	err = json.Unmarshal(b, &resp)
	if err != nil {
		FileLog.Error(err.Error())
		return err
	}
	c.State.SetAll(resp.State)

	FileLog.Info("intialized test case", "id", c.ID, "time", c.Stopped.String())

	// start a ticker
	if c.ticker != nil {
		FileLog.Warn("start called on running simulation")
		return nil
	}

	d := time.Duration(c.updateFreq * int(time.Second))
	c.ticker = time.NewTicker(d)
	FileLog.Debug("ticker started", "interval", d)

	return nil
}

// run should be called on a gorountine and will wait for 1 time step to call
// then start working in a loop.
func (c *TestCase) run() {
	TermLog.Debug("waiting for second time step", "step_duration", c.step)
	for {
		select {
		case <-c.ticker.C:
			inputs := c.writeBuffer.Flush() // may be empty
			// TermLog.Debug("flushed write buffer", "data", inputs)
			newState, err := advance(c.ID, inputs)
			if err != nil {
				FileLog.Error("unable to advance", "test_case", c.ID)
				return
			}
			c.State.SetAll(newState)
			// TermLog.Debug("state update", "new_state", newState)
		case <-c.stopCh:
			err := c.stop()
			if err != nil {
				FileLog.Error("unable to stop", "test_case", c.ID)
			}
			c.stopCh <- struct{}{}
			return
		}
	}
}

func (c *TestCase) SetInput(key string, value any) {
	TermLog.Info("setting input", key, value)
	c.writeBuffer.Set(key, value)
}

// advance takes a testCaseID and a map of inputs to use at the next timestep.
// The map may be empty.
func advance(testCaseID string, inputs map[string]any) (map[string]any, error) {
	url := fmt.Sprintf("http://%s/advance/%s", Host, testCaseID)
	payload, err := json.Marshal(inputs)
	if err != nil {
		return map[string]any{}, err
	}
	TermLog.Debug("making advance request", "payload", string(payload))
	raw := Post(url, "application/json", payload)
	var resp StateUpdate
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		FileLog.Error(err.Error())
		TermLog.Error(err.Error(), "payload", string(raw))
		return nil, err
	}
	return resp.State, nil
}

// func setInputs(testCaseID string, m map[string]string) error {
// 	url := fmt.Sprintf("http://%s/step/%s", Host, testID)
// 	raw, err := Put(url, "application/json", fmt.Appendf([]byte{}, "{\"step\": %d}", step))
// 	if err != nil {
// 		FileLog.Error(err.Error())
// 		return err
// 	}

// 	var resp JSONResponse
// 	err = json.Unmarshal(raw, &resp)
// 	if err != nil {
// 		FileLog.Error(err.Error())
// 		return err
// 	}

// 	return nil
// }

func step(testID string) (int, error) {
	url := fmt.Sprintf("http://%s/step/%s", Host, testID)

	resp, err := Get(url)
	if err != nil {
		FileLog.Error(err.Error())
		return 0, err
	}

	var stepResp SetStepResponse
	err = json.Unmarshal(resp.Body, &stepResp)
	if err != nil {
		FileLog.Error(err.Error())
		return 0, err
	}

	return stepResp.Step, nil
}

func (c *TestCase) Step() (int, error) {
	return step(c.ID)
}

// this should
func setStep(testID string, step int) error {
	url := fmt.Sprintf("http://%s/step/%s", Host, testID)
	raw, err := Put(url, "application/json", fmt.Appendf([]byte{}, "{\"step\": %d}", step))
	if err != nil {
		FileLog.Error(err.Error())
		return err
	}

	var resp JSONResponse
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		FileLog.Error(err.Error())
		return err
	}

	return nil
}

func (c *TestCase) SetStep(step int) error {
	err := setStep(c.ID, step)
	if err != nil {
		return err
	}
	c.step = step
	return nil
}

// True for running, false for an error
func (c *TestCase) Status() bool {
	url := fmt.Sprintf("http://%s/status/%s", Host, c.ID)
	httpResp, err := Get(url)
	if err != nil {
		errMsg := err.Error()
		if strings.HasSuffix(errMsg, "connect: connection refused") {
			FileLog.Error("fatal: boptest server not running")
		}
		return false
	}

	if string(httpResp.Body) != `"Running"` {
		return false
	}

	// fmt.Printf("status is: %s\n", string(httpResp.Body))
	return true
}

// // the only way to see if something is runnig is to use status.
// func Running(name string) bool {
// 	url := fmt.Sprintf("http://%s/name/%s", Host, name)
// 	httpResp, err := Get(url)
// 	if err != nil {
// 		errMsg := err.Error()
// 		if strings.HasSuffix(errMsg, "connect: connection refused") {
// 			FileLog.Error("fatal: boptest server not running")
// 		}
// 		return false
// 	}

// 	var boptestErr ErrorList
// 	err = json.Unmarshal(httpResp.Body, &boptestErr)
// 	if err != nil {
// 		// unmarshalling error
// 		// this may not be an error, could just the testcase is running
// 		FileLog.Info("did not receive error list", "errors", string(httpResp.Body))
// 	}

// 	if len(boptestErr.Errors) > 0 {
// 		// check if the first one indicates that the testcase is not running
// 		FileLog.Info("boptest returned errors", "errors", string(httpResp.Body))
// 		for _, e := range boptestErr.Errors {
// 			if (e.Value == name) && (strings.HasPrefix(e.Msg, "Invalid testid:")) {
// 				// the testcase is not running.
// 				FileLog.Info("testcase not running", "test_case", name)
// 				return false
// 			}
// 		}
// 	}

// 	return false
// }

func TestIdTimeout(testId string) string {
	url := fmt.Sprintf("http://%s/inputs/%s", Host, testId)

	resp, err := http.Get(url)
	if err != nil {
		TermLog.Error(err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		TermLog.Error(err.Error())
	}

	fmt.Printf("'%s'\n", resp.Status)
	fmt.Println(string(body))
	return string(body)
}
