package boptest

import (
	"fmt"
	"testing"
)

var (
	testId   = "28d872d6-d2e9-44b5-9096-29846ed07b47"
	testcase = "bestest_hydronic_heat_pump"
	host     = "0.0.0.0"
)

func TestGetTestId(t *testing.T) {
	Host = host
	GetTestId(testcase)
}

func TestGetMeasurements(t *testing.T) {
	Host = host
	GetMeasurements(testId)
}

func TestGetInputs(t *testing.T) {
	Host = host
	m, err := GetInputs(testId)
	if err != nil {
		t.FailNow()
	}
	for k, p := range m {
		fmt.Printf("%s (%s) '%s'\n", k, p.Unit, p.Description)
	}
}

func TestStopTest(t *testing.T) {
	StopTest(testId)
}
