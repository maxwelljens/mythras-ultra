package world

import (
	"fmt"
	"math"
)

// Duration is a span of in-game time to advance a [Calendar] by. Days and
// Hours are added together (24 Hours make one day), so Duration{Hours: 30}
// is one day and six hours. Negative components are rejected: the calendar
// never runs backwards.
type Duration struct {
	Days  int
	Hours int
}

// Calendar is the in-game date and time of day.
//
// Harness-level neutral calendar: Mythras Imperative defines no calendar
// (digest §8, Time: combat rounds, skill time, and narrative time only), so
// this package uses a neutral 12-month, 30-day year with northern-style
// quarter seasons — chosen at the harness layer, not transcribed from the
// rulebook. Season changes fire as events at quarter boundaries (months
// 1–3 Winter, 4–6 Spring, 7–9 Summer, 10–12 Autumn); TimeOfDay is a
// 24-slot hour label ("00:00" … "23:00") that advances with Hours and never
// emits an event.
type Calendar struct {
	// Year counts from 1; 0 and negatives are invalid state.
	Year int
	// Month is 1–12.
	Month int
	// Day is 1–30.
	Day int
	// Season is the season of the current month ("Winter", "Spring",
	// "Summer", "Autumn"); [Calendar.Advance] recomputes it and repairs
	// stale values.
	Season string
	// TimeOfDay is the hour label from the 24-slot table ("00:00" …
	// "23:00").
	TimeOfDay string
}

// Calendar arithmetic constants (neutral harness-level calendar; see
// [Calendar]).
const (
	monthsPerYear = 12
	daysPerMonth  = 30
)

// seasonsByQuarter maps each three-month quarter of the neutral calendar
// to a season; the strings are the values written to [Calendar.Season].
var seasonsByQuarter = [4]string{"Winter", "Spring", "Summer", "Autumn"}

// timeOfDayByHour is the neutral 24-slot hour table backing
// [Calendar.TimeOfDay]: one slot per hour, so exactly 24 hours of advance
// returns any label to itself (24 is the only slot count with no spurious
// mid-day repeats — every smaller divisor of 24 would repeat a label part-
// way through the day).
var timeOfDayByHour = [24]string{
	"00:00", "01:00", "02:00", "03:00", "04:00", "05:00",
	"06:00", "07:00", "08:00", "09:00", "10:00", "11:00",
	"12:00", "13:00", "14:00", "15:00", "16:00", "17:00",
	"18:00", "19:00", "20:00", "21:00", "22:00", "23:00",
}

// seasonForMonth returns the season of a month in 1..monthsPerYear.
func seasonForMonth(month int) string {
	return seasonsByQuarter[(month-1)/3]
}

// timeOfDayIndex returns the slot of label in timeOfDayByHour, or -1 if the
// label is not one of the 24 valid hours.
func timeOfDayIndex(label string) int {
	for i, slot := range timeOfDayByHour {
		if slot == label {
			return i
		}
	}
	return -1
}

// Advance moves the calendar forward by d and reports the resulting side
// effects as deterministic narration hooks, in order: a year event when the
// year increments, then a season event when the season of the month
// changes. Hours shift TimeOfDay within the 24-slot table and roll into
// days at 24; a time-of-day shift never emits an event. A zero Duration is
// a no-op that returns no events (though a stale Season string is still
// recomputed from the month). Nothing here reads the wall clock.
//
// Errors, on which the calendar is left untouched: a negative duration; a
// duration too large to represent without overflow; an invalid date
// (year < 1, month outside 1–12, day outside 1–30); or, when Hours shift
// it, a TimeOfDay that is not one of the 24 valid hour labels.
func (c *Calendar) Advance(d Duration) ([]string, error) {
	if d.Days < 0 || d.Hours < 0 {
		return nil, fmt.Errorf("world: cannot advance by a negative duration (days %d, hours %d)", d.Days, d.Hours)
	}
	// Overflow guards: totalHours = Days*24 + Hours must stay in int range.
	if d.Days > math.MaxInt/24 || d.Hours > math.MaxInt-d.Days*24 {
		return nil, fmt.Errorf("world: duration too large (days %d, hours %d)", d.Days, d.Hours)
	}
	if c.Year < 1 || c.Month < 1 || c.Month > monthsPerYear || c.Day < 1 || c.Day > daysPerMonth {
		return nil, fmt.Errorf("world: calendar holds an invalid date (year %d, month %d, day %d)",
			c.Year, c.Month, c.Day)
	}
	totalHours := d.Days*24 + d.Hours
	days := totalHours / 24
	remHours := totalHours % 24
	newSlot := -1
	if remHours > 0 {
		newSlot = timeOfDayIndex(c.TimeOfDay)
		if newSlot < 0 {
			return nil, fmt.Errorf("world: unrecognized time of day %q (valid: %s … %s)",
				c.TimeOfDay, timeOfDayByHour[0], timeOfDayByHour[23])
		}
	}

	// All validation has passed; from here the calendar is mutated.
	oldYear, oldMonth := c.Year, c.Month
	if remHours > 0 {
		c.TimeOfDay = timeOfDayByHour[(newSlot+remHours)%24]
	}
	day0 := c.Day - 1 + days // 0-based days from the current month start
	month0 := c.Month - 1 + day0/daysPerMonth
	c.Day = day0%daysPerMonth + 1
	c.Year = oldYear + month0/monthsPerYear
	c.Month = month0%monthsPerYear + 1

	oldSeason := seasonForMonth(oldMonth)
	newSeason := seasonForMonth(c.Month)
	c.Season = newSeason

	var events []string
	if c.Year != oldYear {
		events = append(events, fmt.Sprintf("The year turns: Year %d begins", c.Year))
	}
	if newSeason != oldSeason {
		events = append(events, fmt.Sprintf("Season changes from %s to %s", oldSeason, newSeason))
	}
	return events, nil
}
