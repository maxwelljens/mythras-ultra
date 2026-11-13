package world

import (
	"math"
	"reflect"
	"testing"
)

func TestCalendarAdvance(t *testing.T) {
	tests := []struct {
		name       string
		start      Calendar
		d          Duration
		want       Calendar
		wantEvents []string
	}{
		{
			name:  "zero duration is a no-op",
			start: Calendar{Year: 1, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "08:00"},
			d:     Duration{},
			want:  Calendar{Year: 1, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "08:00"},
		},
		{
			name:  "one day within a month",
			start: Calendar{Year: 1, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "08:00"},
			d:     Duration{Days: 1},
			want:  Calendar{Year: 1, Month: 1, Day: 2, Season: "Winter", TimeOfDay: "08:00"},
		},
		{
			name:  "hours shift time of day only",
			start: Calendar{Year: 1, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "08:00"},
			d:     Duration{Hours: 2},
			want:  Calendar{Year: 1, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "10:00"},
		},
		{
			name:  "24 hours rolls a day and returns the hour",
			start: Calendar{Year: 1, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "08:00"},
			d:     Duration{Hours: 24},
			want:  Calendar{Year: 1, Month: 1, Day: 2, Season: "Winter", TimeOfDay: "08:00"},
		},
		{
			name:  "30 hours is one day and six hours",
			start: Calendar{Year: 1, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "08:00"},
			d:     Duration{Hours: 30},
			want:  Calendar{Year: 1, Month: 1, Day: 2, Season: "Winter", TimeOfDay: "14:00"},
		},
		{
			name:  "hours wrap past midnight",
			start: Calendar{Year: 1, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "22:00"},
			d:     Duration{Hours: 5},
			want:  Calendar{Year: 1, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "03:00"},
		},
		{
			name:  "month rollover without season change",
			start: Calendar{Year: 1, Month: 1, Day: 30, Season: "Winter", TimeOfDay: "08:00"},
			d:     Duration{Days: 1},
			want:  Calendar{Year: 1, Month: 2, Day: 1, Season: "Winter", TimeOfDay: "08:00"},
		},
		{
			name:  "60 days lands three months on",
			start: Calendar{Year: 1, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "08:00"},
			d:     Duration{Days: 60},
			want:  Calendar{Year: 1, Month: 3, Day: 1, Season: "Winter", TimeOfDay: "08:00"},
		},
		{
			name:       "season change at quarter boundary",
			start:      Calendar{Year: 1, Month: 3, Day: 30, Season: "Winter", TimeOfDay: "08:00"},
			d:          Duration{Days: 1},
			want:       Calendar{Year: 1, Month: 4, Day: 1, Season: "Spring", TimeOfDay: "08:00"},
			wantEvents: []string{"Season changes from Winter to Spring"},
		},
		{
			name:       "year rollover fires year then season events",
			start:      Calendar{Year: 1, Month: 12, Day: 30, Season: "Autumn", TimeOfDay: "08:00"},
			d:          Duration{Days: 1},
			want:       Calendar{Year: 2, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "08:00"},
			wantEvents: []string{"The year turns: Year 2 begins", "Season changes from Autumn to Winter"},
		},
		{
			name:       "days across the year boundary",
			start:      Calendar{Year: 1, Month: 12, Day: 15, Season: "Autumn", TimeOfDay: "08:00"},
			d:          Duration{Days: 20},
			want:       Calendar{Year: 2, Month: 1, Day: 5, Season: "Winter", TimeOfDay: "08:00"},
			wantEvents: []string{"The year turns: Year 2 begins", "Season changes from Autumn to Winter"},
		},
		{
			name:  "stale season string is repaired from the month",
			start: Calendar{Year: 1, Month: 1, Day: 1, Season: "Summer", TimeOfDay: "08:00"},
			d:     Duration{Days: 1},
			want:  Calendar{Year: 1, Month: 1, Day: 2, Season: "Winter", TimeOfDay: "08:00"},
		},
		{
			name:  "days only preserve an unconventional time of day",
			start: Calendar{Year: 1, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "dusk"},
			d:     Duration{Days: 3},
			want:  Calendar{Year: 1, Month: 1, Day: 4, Season: "Winter", TimeOfDay: "dusk"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.start
			events, err := c.Advance(tc.d)
			if err != nil {
				t.Fatalf("Advance(%+v): %v", tc.d, err)
			}
			if c != tc.want {
				t.Errorf("calendar = %+v, want %+v", c, tc.want)
			}
			if !reflect.DeepEqual(events, tc.wantEvents) {
				t.Errorf("events = %#v, want %#v", events, tc.wantEvents)
			}
		})
	}
}

func TestCalendarAdvanceErrors(t *testing.T) {
	valid := Calendar{Year: 1, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "08:00"}
	tests := []struct {
		name  string
		start Calendar
		d     Duration
	}{
		{"negative days", valid, Duration{Days: -1}},
		{"negative hours", valid, Duration{Hours: -1}},
		{"days overflow", valid, Duration{Days: math.MaxInt/24 + 1}},
		{"hours overflow", valid, Duration{Days: math.MaxInt / 24, Hours: math.MaxInt}},
		{"invalid year", Calendar{Year: 0, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "08:00"}, Duration{Days: 1}},
		{"invalid month zero", Calendar{Year: 1, Month: 0, Day: 1, Season: "Winter", TimeOfDay: "08:00"}, Duration{Days: 1}},
		{"invalid month high", Calendar{Year: 1, Month: 13, Day: 1, Season: "Winter", TimeOfDay: "08:00"}, Duration{Days: 1}},
		{"invalid day zero", Calendar{Year: 1, Month: 1, Day: 0, Season: "Winter", TimeOfDay: "08:00"}, Duration{Days: 1}},
		{"invalid day high", Calendar{Year: 1, Month: 1, Day: 31, Season: "Winter", TimeOfDay: "08:00"}, Duration{Days: 1}},
		{"bad time of day with hours", Calendar{Year: 1, Month: 1, Day: 1, Season: "Winter", TimeOfDay: "high noon"}, Duration{Hours: 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.start
			events, err := c.Advance(tc.d)
			if err == nil {
				t.Fatalf("Advance(%+v) succeeded, want error", tc.d)
			}
			if events != nil {
				t.Errorf("events = %v, want nil on error", events)
			}
			if c != tc.start {
				t.Errorf("error mutated the calendar: %+v, want unchanged %+v", c, tc.start)
			}
		})
	}
}

func TestSeasonForMonth(t *testing.T) {
	want := []string{
		"Winter", "Winter", "Winter",
		"Spring", "Spring", "Spring",
		"Summer", "Summer", "Summer",
		"Autumn", "Autumn", "Autumn",
	}
	for month := 1; month <= 12; month++ {
		if got := seasonForMonth(month); got != want[month-1] {
			t.Errorf("seasonForMonth(%d) = %q, want %q", month, got, want[month-1])
		}
	}
}

func TestTimeOfDayIndex(t *testing.T) {
	if got := timeOfDayIndex("08:00"); got != 8 {
		t.Errorf("timeOfDayIndex(08:00) = %d, want 8", got)
	}
	if got := timeOfDayIndex("23:00"); got != 23 {
		t.Errorf("timeOfDayIndex(23:00) = %d, want 23", got)
	}
	if got := timeOfDayIndex("24:00"); got != -1 {
		t.Errorf("timeOfDayIndex(24:00) = %d, want -1", got)
	}
	if got := timeOfDayIndex(""); got != -1 {
		t.Errorf("timeOfDayIndex(\"\") = %d, want -1", got)
	}
}
