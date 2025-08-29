package boptest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"maps"
)

var (
	Host = "0.0.0.0"

	termLogLevel = new(slog.LevelVar)
	fileLogLevel = new(slog.LevelVar)

	termLog *slog.Logger
	fileLog *slog.Logger
)

const (
	HTTPStatus_Ok         = "200 OK"
	HTTPStatus_BadRequest = "400 Bad Request"

	ContentType_ApplicationJSON = "application/json"

	DefaultStep = 3600 // 1 hour, per BOPTEST
)

// called when the package is imported
func init() {
	// You can set the logging level programmatically, from a config file, or env variable.
	termLogLevel.Set(slog.LevelDebug)
	fileLogLevel.Set(slog.LevelDebug)

	// standard terminal logger
	termLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: termLogLevel, // this can be set programmatically
	}))

	// create a file, if it doesn't exist, and write json log there
	file, err := os.OpenFile("boptest_log.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		termLog.Error("Failed to open log file", "error", err)
		os.Exit(1)
	}
	// assign the terminal logger
	fileLog = slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: fileLogLevel, // this can be set programmatically
	}))
}

// a concurrency safe map for storing simulation state
type StateMap struct {
	state map[string]any

	sync.RWMutex
}

// overwrites the whole map
func (m *StateMap) SetAll(newState map[string]any) {
	m.Lock()
	defer m.Unlock()

	m.state = make(map[string]any, len(newState))
	maps.Copy(m.state, newState)
}

func (m *StateMap) GetAll() map[string]any {
	m.RLock()
	defer m.RUnlock()

	copy := make(map[string]any, len(m.state))
	maps.Copy(copy, m.state)

	return copy
}

// returns int, float, bool, string, or nil, if not present
func (m *StateMap) Get(key string) any {
	m.RLock()
	defer m.RUnlock()
	if val, ok := m.state[key]; ok {
		return val
	}
	return nil
}

func (m *StateMap) GetMultiple(keys []string) map[string]any {
	m.RLock()
	defer m.RUnlock()

	results := make(map[string]any)
	for _, key := range keys {
		if val, ok := m.state[key]; ok {
			results[key] = val
		}
	}
	return results
}

type TestCase struct {
	ID string `json:"testid"`

	ticker *time.Ticker  `json:"-"`
	stopCh chan struct{} `json:"-"`

	Created time.Time `json:"-"`
	Started time.Time `json:"-"`
	Stopped time.Time `json:"-"`

	step int `json:"-"` // increment of time to advance the simulation by

	StartTime int `json:"start_time"`    // seconds since start of year
	WarmUp    int `json:"warmup_period"` // seconds before startTime

	State StateMap `json:"-"`
}

type testCaseOption func(*TestCase)

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

type HTTPResponse struct {
	Status string
	Body   []byte
}

type StateUpdate struct {
	JSONResponse
	State map[string]any `json:"payload"`
}

type SetStepResponse struct {
	JSONResponse
	Step int `json:"payload"`
}

func Get(url string) (HTTPResponse, error) {
	resp, err := http.Get(url)
	if err != nil {
		termLog.Error(err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		termLog.Error(err.Error())
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
		termLog.Error(err.Error())
	}
	return body, nil
}

func Post(url, contentType string, payload []byte) []byte {
	postBody := bytes.NewBuffer(payload)
	resp, err := http.Post(url, contentType, postBody)
	if err != nil {
		termLog.Error(err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		termLog.Error(err.Error())
	}
	return body
}

// takes the name of the testcase and returns the test id.
func NewTestCase(testcase string, opts ...testCaseOption) (*TestCase, error) {
	url := fmt.Sprintf("http://%s/testcases/%s/select", Host, testcase)

	postBody := bytes.NewBuffer([]byte{})
	resp, err := http.Post(url, "text/raw", postBody)
	if err != nil {
		termLog.Error(err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		termLog.Error(err.Error())
	}

	// unmarshal
	var testCase *TestCase
	err = json.Unmarshal(body, &testCase)
	if err != nil {
		termLog.Error(err.Error())
		return testCase, err
	}

	// initialize fields
	testCase.State = StateMap{
		state: make(map[string]any),
	}
	testCase.step = DefaultStep
	testCase.stopCh = make(chan struct{})
	testCase.Created = time.Now()

	fileLog.Info("created test case", "id", testCase.ID, "time", testCase.Created.String())

	// apply optional parameters
	for _, opt := range opts {
		opt(testCase)
	}

	return testCase, nil
}

func stopTestCase(testId string) error {
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
	err := stopTestCase(c.ID)
	if err != nil {
		return err
	}
	c.Stopped = time.Now()
	fileLog.Info("stopped test case", "id", c.ID, "time", c.Stopped.String())
	return nil
}

func (c *TestCase) Stop() {
	c.stopCh <- struct{}{}
}

// takes the testid and returns all possible measurements
func measurements(testid string) (map[string]PointProperties, error) {
	url := fmt.Sprintf("http://%s/measurements/%s", Host, testid)
	// fmt.Println(url)

	resp, err := http.Get(url)
	if err != nil {
		termLog.Error(err.Error())
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

func (c *TestCase) Start() error {
	// set the step if its not the default
	if c.step != DefaultStep {
		err := c.SetStep(c.step)
		if err != nil {
			fileLog.Error("unable to set step", "test_case", c.ID)
			return err
		}
	}

	// define t=0 and start simulation
	payload, err := json.Marshal(c)
	if err != nil {
		fileLog.Error(err.Error())
		return err
	}
	// fmt.Printf("intializing with\n%s\n", string(payload))

	url := fmt.Sprintf("http://%s/initialize/%s", Host, c.ID)
	b, err := Put(url, "application/json", payload)
	if err != nil {
		fileLog.Error(err.Error())
		return err
	}

	// fmt.Println(string(b))
	var resp StateUpdate
	err = json.Unmarshal(b, &resp)
	if err != nil {
		fileLog.Error(err.Error())
		return err
	}
	c.State.SetAll(resp.State)

	fileLog.Info("intialized test case", "id", c.ID, "time", c.Stopped.String())

	// start a ticker
	if c.ticker != nil {
		fileLog.Warn("start called on running simulation")
		return nil
	}
	d := time.Duration(c.step * int(time.Second))
	c.ticker = time.NewTicker(d)
	fileLog.Debug("ticker started", "interval", d)
	// wait for c.step second then launch run
	go c.run()

	return nil
}

// run should be called on a gorountine and will wait for 1 time step to call
// then start working in a loop.
func (c *TestCase) run() {
	termLog.Debug("waiting for second time step", "step_duration", c.step)
	for {
		select {
		case <-c.ticker.C:
			newState, err := advance(c.ID)
			if err != nil {
				fileLog.Error("unable to advance", "test_case", c.ID)
				return
			}
			c.State.SetAll(newState)

		case <-c.stopCh:
			err := c.stop()
			if err != nil {
				fileLog.Error("unable to stop", "test_case", c.ID)
				return
			}
			return
		}
	}
}

func advance(testCaseID string) (map[string]any, error) {
	termLog.Debug("tick")

	url := fmt.Sprintf("http://%s/advance/%s", Host, testCaseID)
	raw := Post(url, "application/json", []byte("{}"))

	var resp StateUpdate
	err := json.Unmarshal(raw, &resp)
	if err != nil {
		fileLog.Error(err.Error())
		return nil, err
	}

	return resp.State, nil
}

func step(testID string) (int, error) {
	url := fmt.Sprintf("http://%s/step/%s", Host, testID)

	resp, err := Get(url)
	if err != nil {
		fileLog.Error(err.Error())
		return 0, err
	}

	var stepResp SetStepResponse
	err = json.Unmarshal(resp.Body, &stepResp)
	if err != nil {
		fileLog.Error(err.Error())
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
	raw, err := Put(url, "application/json", []byte(fmt.Sprintf("{\"step\": %d}", step)))
	if err != nil {
		fileLog.Error(err.Error())
		return err
	}

	var resp JSONResponse
	err = json.Unmarshal(raw, &resp)
	if err != nil {
		fileLog.Error(err.Error())
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

func TestIdTimeout(testId string) string {
	url := fmt.Sprintf("http://%s/inputs/%s", Host, testId)

	resp, err := http.Get(url)
	if err != nil {
		termLog.Error(err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		termLog.Error(err.Error())
	}

	fmt.Printf("'%s'\n", resp.Status)
	fmt.Println(string(body))
	return string(body)
}
