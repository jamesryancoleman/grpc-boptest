package boptest

import (
	"fmt"
	"log/slog"
	"testing"
)

var (
	testID   = "c07abb5f-7f21-4d1f-a325-bba1ce05f7b1"
	testcase = "bestest_air"
	host     = "0.0.0.0:1025"
)

// func TestLaunchTestCase(t *testing.T) {
// 	Host = host
// 	testID, err := NewTestCase(testcase)
// 	if err != nil {
// 		log.Println(err.Error())
// 		t.FailNow()
// 	}
// 	fmt.Printf("started test case \"%s\" with id \"%s\".\n", testcase, testID)
// }

func TestLoggingLevels(t *testing.T) {
	fmt.Printf("The term log level is %s\n", termLogLevel.Level().String())
	termLogLevel.Set(slog.LevelDebug)
	fmt.Printf("The term log level is %s\n", termLogLevel.Level().String())
	fileLogLevel.Set(slog.LevelWarn)
	fmt.Printf("The file log level is %s\n", fileLogLevel.Level().String())
	termLog.Info("this should print to the terminal")
	fileLog.Info("this should print log file")
}

func TestStopTest(t *testing.T) {
	fmt.Printf("stopping test case \"%s\"\n", testID)
	stopTestCase(testID)
}

func TestStartStopTestCase(t *testing.T) {
	Host = host
	testCase, err := NewTestCase(testcase)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	// stop the testcase
	err = testCase.Stop()
	if err != nil {
		fmt.Println(err.Error())
		fmt.Printf("error: could not stop test case \"%s\" with id \"%s\". Please stop mannually.\n",
			testcase, testCase.ID)
		t.FailNow()
	}
}

func startTestCase() *TestCase {
	Host = host
	testCase, err := NewTestCase(testcase)
	if err != nil {
		fileLog.Error(err.Error())
		return nil
	}
	// fmt.Printf("started test case \"%s\" with id \"%s\" @ %v.\n",
	// 	testcase, testCase.ID, testCase.Created)
	return testCase
}

func TestMeasurements(t *testing.T) {
	testCase := startTestCase()
	if testCase == nil {
		t.FailNow()
	}
	defer testCase.Stop()

	m, err := testCase.Measurements()
	if err != nil {
		fileLog.Error(err.Error())
		t.FailNow()
	}
	for k, p := range m {
		fmt.Printf("%s (%s) '%s'\n", k, p.Unit, p.Description)
	}
}

func TestGetInputs(t *testing.T) {
	testCase := startTestCase()
	if testCase == nil {
		t.FailNow()
	}
	defer testCase.Stop()

	m, err := testCase.Inputs()
	if err != nil {
		t.FailNow()
	}
	for k, p := range m {
		fmt.Printf("%s (%s) '%s'\n", k, p.Unit, p.Description)
	}
}

func TestRunTestCase(t *testing.T) {
	Host = host
	testCase, err := NewTestCase(testcase, WithStartTime(3600*24*31))
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
	defer testCase.Stop()

	err = testCase.Run()
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
}

func TestSetGetStep(t *testing.T) {
	Host = host
	testCase, err := NewTestCase(testcase, WithStartTime(3600*24*31))
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
	defer testCase.Stop()

	err = testCase.Run()
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	err = testCase.SetStep(60) // seconds
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	s, err := testCase.Step()
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
	fmt.Printf("the time step is set to %d seconds", s)
}
