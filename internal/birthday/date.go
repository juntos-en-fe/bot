package birthday

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Date is a recurring birthday date without a year.
type Date struct {
	Month time.Month
	Day   int
}

// ParseDate accepts a birthday in DD/MM form, including 29/02.
func ParseDate(input string) (Date, error) {
	parts := strings.Split(strings.TrimSpace(input), "/")
	if len(parts) != 2 {
		return Date{}, fmt.Errorf("la fecha debe tener el formato DD/MM")
	}

	day, dayErr := strconv.Atoi(parts[0])
	month, monthErr := strconv.Atoi(parts[1])
	if dayErr != nil || monthErr != nil {
		return Date{}, fmt.Errorf("la fecha debe tener el formato DD/MM")
	}

	date := Date{Month: time.Month(month), Day: day}
	if !date.Valid() {
		return Date{}, fmt.Errorf("la fecha %q no es válida", input)
	}
	return date, nil
}

// Valid reports whether the day exists in at least one year. Leap day is valid.
func (d Date) Valid() bool {
	if d.Month < time.January || d.Month > time.December || d.Day < 1 {
		return false
	}
	validated := time.Date(2000, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
	return validated.Month() == d.Month && validated.Day() == d.Day
}

func (d Date) String() string {
	return fmt.Sprintf("%02d/%02d", d.Day, d.Month)
}
