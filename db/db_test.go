package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_NowTimestamp(t *testing.T) {
	startTime := time.Now()
	time.Sleep(5 * time.Millisecond)

	actual := NowTimestamp()

	time.Sleep(5 * time.Millisecond)
	endTime := time.Now()

	assert := assert.New(t)
	assert.Less(startTime, actual.Time(), "actual timestamp not after start")
	assert.Less(actual.Time(), endTime, "actual timestamp not before end")
}

func Test_Timestamp_Scan(t *testing.T) {

}

func Test_Timestamp_Value(t *testing.T) {

}

func Test_Email_Scan(t *testing.T) {

}

func Test_Email_Value(t *testing.T) {

}
