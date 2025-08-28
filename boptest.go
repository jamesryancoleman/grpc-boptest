package boptest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
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
)

// called when the package is imported
func init() {
	// You can set the logging level programmatically, from a config file, or env variable.

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

type TestCase struct {
	ID string `json:"testid"`

	Created time.Time
	Started time.Time
	Stopped time.Time

	StartTime int `json:"start_time"`    // seconds since start of year
	WarmUp    int `json:"warmup_period"` // seconds before startTime
}

type testCaseOption func(*TestCase)

// seconds since start of year
func WithStartTime(t int) testCaseOption {
	return func(tcc *TestCase) {
		tcc.StartTime = t
	}
}

// seconds before startTime
func WithWarmUp(d int) testCaseOption {
	return func(tcc *TestCase) {
		tcc.WarmUp = d
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

type IntializeResponse struct {
	JSONResponse
	State map[string]any `json:"payload"`
}

type StepResponse struct {
	JSONResponse
	Step int `json:"payload"`
}

// type State struct {
// 	Time   float64        `json:"time"`
// 	Fields map[string]any `json:"-"`
// }

// func (s *State) UnmarshalJSON(data []byte) error {
// 	var m map[string]any
// 	err := json.Unmarshal(data, &m)
// 	if err != nil {
// 		return err
// 	}
// 	if val, ok := m["time"]; ok {
// 		fmt.Printf("%v (%T)\n", val, val)
// 		if t, ok := val.(float64); ok {
// 			s.Time = t
// 		}
// 		delete(m, "time")
// 	}
// 	s.Fields = m
// 	return nil

// }

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
	testCase.Created = time.Now()
	fileLog.Info("started test case", "id", testCase.ID, "time", testCase.Created.String())

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

func (c *TestCase) Stop() error {
	err := stopTestCase(c.ID)
	if err != nil {
		return err
	}
	c.Stopped = time.Now()
	fileLog.Info("stopped test case", "id", c.ID, "time", c.Stopped.String())
	return nil
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

func (c *TestCase) Run() error {
	url := fmt.Sprintf("http://%s/initialize/%s", Host, c.ID)

	payload, err := json.Marshal(c)
	if err != nil {
		fileLog.Error(err.Error())
		return err
	}

	// fmt.Printf("intializing with\n%s\n", string(payload))

	b, err := Put(url, "application/json", payload)
	if err != nil {
		fileLog.Error(err.Error())
		return err
	}

	// fmt.Println(string(b))

	var resp IntializeResponse
	err = json.Unmarshal(b, &resp)
	if err != nil {
		fileLog.Error(err.Error())
		return err
	}

	// fmt.Printf("status: %d\nmsg: %s\n", resp.Status, resp.Message)
	// fmt.Printf("state: \n")
	// for k, v := range resp.State {
	// 	fmt.Printf("\t%s: %v\n", k, v)
	// }

	return nil
}

func step(testID string) (int, error) {
	url := fmt.Sprintf("http://%s/step/%s", Host, testID)

	resp, err := Get(url)
	if err != nil {
		fileLog.Error(err.Error())
		return 0, err
	}

	var stepResp StepResponse
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
	return setStep(c.ID, step)
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
