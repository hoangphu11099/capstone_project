package controllers

import (
	"student-management/models"
	"time"
)

// Concrete teaching dates take precedence over semester dates when deciding
// whether teaching has finished. Future classes can prepare assignments early.
func exerciseOfferingEnded(offering models.CourseOffering, now time.Time) bool {
	if offering.Status != "open" || offering.Class.Status == "closed" {
		return true
	}
	hasLesson := false
	for _, schedule := range offering.Schedules {
		var date time.Time
		if schedule.FullDate != nil {
			date = *schedule.FullDate
		} else {
			if offering.Semester.EndDate.IsZero() {
				continue
			}
			name, ok := normalizeScheduleDay(schedule.DayOfWeek)
			if !ok {
				continue
			}
			date = offering.Semester.EndDate
			for i := 0; i < 7 && date.Format("Mon") != name; i++ {
				date = date.AddDate(0, 0, -1)
			}
			if !offering.Semester.StartDate.IsZero() && date.Before(offering.Semester.StartDate) {
				continue
			}
		}
		end := schedule.EndTime
		if end == "" {
			end = "23:59"
		}
		if len(end) > 5 {
			end = end[:5]
		}
		instant, err := time.ParseInLocation("2006-01-02 15:04", date.Format("2006-01-02")+" "+end, attendanceLocation())
		if err != nil {
			continue
		}
		hasLesson = true
		if instant.After(now) {
			return false
		}
	}
	if hasLesson {
		return true
	}
	return offering.Semester.Status == "closed" || (!offering.Semester.EndDate.IsZero() && offering.Semester.EndDate.Format("2006-01-02") < now.In(attendanceLocation()).Format("2006-01-02"))
}
