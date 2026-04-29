package raceplan

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/tormoder/fit"
)

func TestPlanBytesBuildsRacePlanFromCourseFIT(t *testing.T) {
	data := buildCourseFIT(t)

	plan, err := PlanBytes("sample-course.fit", data, Profile{
		FTPWatts:        285,
		WeightKG:        70,
		MaxCarbGPerHour: 90,
		BottleML:        750,
		StartBottles:    2,
		CaffeineMgPerKG: 3,
		StrategyMode:    "balanced",
	})
	if err != nil {
		t.Fatalf("PlanBytes() error: %v", err)
	}

	if plan.SourceType != "course" {
		t.Fatalf("unexpected source type: %q", plan.SourceType)
	}
	if plan.DistanceMeters < 10000 {
		t.Fatalf("expected route distance, got %.1f m", plan.DistanceMeters)
	}
	if len(plan.Climbs) == 0 {
		t.Fatal("expected at least one detected climb")
	}
	if len(plan.DecisiveSectors) == 0 {
		t.Fatal("expected decisive sectors")
	}
	if len(plan.FuelPlan.Checkpoints) == 0 {
		t.Fatal("expected fueling checkpoints")
	}
	if len(plan.Strategy) == 0 {
		t.Fatal("expected tactical strategy sections")
	}

	md := BuildMarkdown(plan)
	if !strings.Contains(md, "# Race Plan") {
		t.Fatalf("markdown missing heading: %q", md)
	}
	if !strings.Contains(md, "Key Climbs") {
		t.Fatalf("markdown missing climbs section: %q", md)
	}
}

func buildCourseFIT(t *testing.T) []byte {
	t.Helper()

	header := fit.NewHeader(fit.V20, true)
	file, err := fit.NewFile(fit.FileTypeCourse, header)
	if err != nil {
		t.Fatalf("new fit file: %v", err)
	}

	course, err := file.Course()
	if err != nil {
		t.Fatalf("course accessor: %v", err)
	}
	course.Course = fit.NewCourseMsg()
	course.Course.Name = "Synthetic Road Race"
	course.Course.Sport = fit.SportCycling
	course.Course.SubSport = fit.SubSportRoad

	start := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	lat := 42.0
	lon := -83.0
	for i := 0; i <= 28; i++ {
		distM := float64(i) * 500
		altM := 180.0
		switch {
		case distM >= 5000 && distM <= 8000:
			altM = 180 + ((distM - 5000) / 3000 * 180)
		case distM > 8000 && distM <= 10000:
			altM = 360 - ((distM - 8000) / 2000 * 80)
		case distM > 10000:
			altM = 280 + ((distM - 10000) / 4000 * 30)
		}

		record := fit.NewRecordMsg()
		record.Timestamp = start.Add(time.Duration(i) * 75 * time.Second)
		record.Distance = uint32(distM * 100)
		record.EnhancedAltitude = uint32((altM + 500) * 5)
		record.PositionLat = fit.NewLatitudeDegrees(lat + (distM / 111320.0))
		record.PositionLong = fit.NewLongitudeDegrees(lon + (float64(i%3) * 0.0002))
		course.Records = append(course.Records, record)
	}

	aid := fit.NewCoursePointMsg()
	aid.Distance = uint32(9000 * 100)
	aid.Type = fit.CoursePointAidStation
	aid.Name = "Aid Station"
	aid.Timestamp = start.Add(25 * time.Minute)
	aid.PositionLat = fit.NewLatitudeDegrees(42.081)
	aid.PositionLong = fit.NewLongitudeDegrees(-82.999)
	course.CoursePoints = append(course.CoursePoints, aid)

	turn := fit.NewCoursePointMsg()
	turn.Distance = uint32(13200 * 100)
	turn.Type = fit.CoursePointSharpRight
	turn.Name = "Sharp Right"
	turn.Timestamp = start.Add(36 * time.Minute)
	turn.PositionLat = fit.NewLatitudeDegrees(42.118)
	turn.PositionLong = fit.NewLongitudeDegrees(-82.998)
	course.CoursePoints = append(course.CoursePoints, turn)

	var buf bytes.Buffer
	if err := fit.Encode(&buf, file, binary.LittleEndian); err != nil {
		t.Fatalf("encode fit: %v", err)
	}
	return buf.Bytes()
}
