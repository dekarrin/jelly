package jelsort

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_By(t *testing.T) {
	assert := assert.New(t)

	expect := []string{
		"entry-1-info",
		"alpha-2-info",
		"delta-7",
		"max-16-info",
	}

	input := []string{
		"alpha-2-info",
		"entry-1-info",
		"max-16-info",
		"delta-7",
	}

	actual := By(input, func(left, right string) bool {
		leftParts := strings.Split(left, "-")
		rightParts := strings.Split(right, "-")

		if len(leftParts) < 2 || len(rightParts) < 2 {
			// no number, so just assume true
			return true
		}

		leftNumStr := leftParts[1]
		rightNumStr := rightParts[1]

		leftNum, err := strconv.Atoi(leftNumStr)
		if err != nil {
			return true
		}

		rightNum, err := strconv.Atoi(rightNumStr)
		if err != nil {
			return true
		}

		return leftNum < rightNum
	})

	assert.Equal(expect, actual)
}
