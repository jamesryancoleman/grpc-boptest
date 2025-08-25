package boptest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

var (
	Host = "localhost"
)

const (
	HTTPStatus_Ok         = "200 OK"
	HTTPStatus_BadRequest = "400 Bad Request"

	ContentType_ApplicationJSON = "application/json"
)

type TestInstance struct {
	Id string `json:"testid"`
}

type APIResponse struct {
	Status  int                        `json:"status"`
	Message string                     `json:"message"`
	Payload map[string]PointProperties `json:"payload"`
}

type PointProperties struct {
	Unit        string
	Description string
	Minimum     float64
	Maximum     float64
}

type Response struct {
	Status string
	Body   []byte
}

func Get(url string) (Response, error) {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	return Response{
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
		log.Fatal(err)
	}
	return body, nil
}

func Post(url, contentType string, payload []byte) []byte {
	postBody := bytes.NewBuffer(payload)
	resp, err := http.Post(url, contentType, postBody)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	return body
}

// takes the name of the testcase and returns the test id.
func GetTestId(testcase string) string {
	url := fmt.Sprintf("http://%s/testcases/%s/select", Host, testcase)
	// url := fmt.Sprintf("http://%s/testcases", Host)
	fmt.Println(url)

	postBody := bytes.NewBuffer([]byte{})
	resp, err := http.Post(url, "text/raw", postBody)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.Status, string(body))
	return string(body)
}

// takes the testid and returns all possible measurements
func GetMeasurements(testid string) (string, error) {
	url := fmt.Sprintf("http://%s/measurements/%s", Host, testid)
	fmt.Println(url)

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.Status == HTTPStatus_BadRequest {
		fmt.Printf("'%s'\n", resp.Status) // turn this to a log
		return "", fmt.Errorf("%s", resp.Status)
	}

	// if resp.Status == HTTPStatus_Ok {
	// }
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil
	}

	var r APIResponse
	if err := json.Unmarshal(body, &r); err != nil {
		panic(err)
	}
	// fmt.Printf("%+v\n", r)
	for k, p := range r.Payload {
		fmt.Printf("%s (%s) '%s'\n", k, p.Unit, p.Description)
	}

	// fmt.Printf("'%s'\n", resp.Status)
	// fmt.Println(string(body))
	return string(body), nil
}

func GetInputs(testid string) (map[string]PointProperties, error) {
	url := fmt.Sprintf("http://%s/inputs/%s", Host, testid)
	fmt.Println(url)

	resp, err := Get(url)
	if err != nil {
		return nil, err
	}

	var r APIResponse
	if err := json.Unmarshal(resp.Body, &r); err != nil {
		panic(err)
	}
	return r.Payload, nil
}

func TestIdTimeout(testId string) string {
	url := fmt.Sprintf("http://%s/inputs/%s", Host, testId)
	fmt.Println(url)

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("'%s'\n", resp.Status)
	fmt.Println(string(body))
	return string(body)
}

func StopTest(testId string) error {
	url := fmt.Sprintf("http://%s/stop/%s", Host, testId)
	resp, err := Put(url, "", []byte{})
	if err != nil {
		return err
	}
	// fmt.Printf("'%s'\n", resp.Status)
	fmt.Println(string(resp))
	return nil
}
