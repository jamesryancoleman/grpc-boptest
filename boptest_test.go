package boptest

import (
	"fmt"
	"log/slog"
	"testing"
	"time"
)

var (
	testID   = "61df7871-dabe-4383-805a-50413c991b85"
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
	Host = host
	fmt.Printf("stopping test case \"%s\"\n", testID)
	err := stopTestCase(testID)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
}

func TestCreateAndStopTestCase(t *testing.T) {
	Host = host
	testCase, err := NewTestCase(testcase)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	// stop the testcase
	err = testCase.stop()
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
	defer testCase.stop()

	m, err := testCase.Inputs()
	if err != nil {
		t.FailNow()
	}
	for k, p := range m {
		fmt.Printf("%s (%s) '%s'\n", k, p.Unit, p.Description)
	}
}

func TestStopWithChannel(t *testing.T) {
	Host = host
	testCase, err := NewTestCase(testcase,
		WithStartTime(3600*24*31),
		WithStep(2), // seconds
	)
	if err != nil {
		fileLog.Error(err.Error())
	}

	err = testCase.Start()
	if err != nil {
		fileLog.Error(err.Error())
	}

	fmt.Println("sleeping until stop channel send")
	time.Sleep(time.Second * 7)
	testCase.Stop()

	time.Sleep(time.Second * 1) // wait to see clean up

}

func TestRunTestCase(t *testing.T) {
	Host = host
	testCase, err := NewTestCase(testcase, WithStartTime(3600*24*31))
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
	defer testCase.Stop()

	err = testCase.Start()
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
}

func TestBreakLoop(t *testing.T) {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

loop:
	for {
		select {
		case <-timer.C:
			break loop
		default:
			// Do work
			fmt.Println("doing work")
			time.Sleep(100 * time.Millisecond)
		}
	}
	fmt.Println("exited loop")

}

func TestGetMultiple(t *testing.T) {
	fileLogLevel.Set(slog.LevelDebug)
	termLogLevel.Set(slog.LevelDebug)
	Host = host

	testCase, err := NewTestCase(testcase,
		WithStartTime(3600*24*31),
		WithStep(2),
	)
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	err = testCase.Start()
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	timeKey := "time"
	ptKey := "zon_reaTRooAir_y"

	func() {
		timeout := time.After(5 * time.Second)
		for {
			select {
			case <-timeout:
				return
			default:
				m := testCase.State.GetMultiple([]string{timeKey, ptKey})
				fmt.Printf("%v\n", m)
				time.Sleep(2 * time.Second)
			}
		}
	}()

	testCase.Stop()
	time.Sleep(time.Second * 1) // wait for cleanup
}

func TestSetGetStep(t *testing.T) {
	Host = host
	testCase, err := NewTestCase(testcase, WithStartTime(3600*24*31))
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
	defer testCase.Stop()

	// confirm default step size
	s, err := testCase.Step()
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
	fmt.Printf("default time step is %d seconds\n", s)

	// set up the step size before running
	err = testCase.SetStep(60) // seconds
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}

	s, err = testCase.Step()
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
	fmt.Printf("the time step is set to %d seconds\n", s)

	err = testCase.Start()
	if err != nil {
		fmt.Println(err.Error())
		t.FailNow()
	}
}
