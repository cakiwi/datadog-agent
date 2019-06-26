package api

import (
	"bytes"
	"math/rand"
	"strings"
	"testing"
	"unicode"

	"github.com/DataDog/datadog-agent/pkg/trace/info"
	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/stretchr/testify/assert"
)

func newTestSpan() *pb.Span {
	return &pb.Span{
		Duration: 10000000,
		Error:    0,
		Resource: "GET /some/raclette",
		Service:  "django",
		Name:     "django.controller",
		SpanID:   rand.Uint64(),
		Start:    1448466874000000000,
		TraceID:  424242,
		Meta: map[string]string{
			"user": "leo",
			"pool": "fondue",
		},
		Metrics: map[string]float64{
			"cheese_weight": 100000.0,
		},
		ParentID: 1111,
		Type:     "http",
	}
}

// statsToIssues returns map of {reason->count} for normalization issues due to dropped or malformed traces
func statsToIssues(ts *info.TagStats) map[string]int64 {
	m := ts.TracesDropped.TagValues()
	for r, c := range ts.TracesMalformed.TagValues() {
		m[r] = c
	}
	return m
}

// noIssues returns map of {reason->count} for normalization issues, with all counts=0, meaning there were no issues
func noIssues() map[string]int64 {
	return statsToIssues(&info.TagStats{})
}

// onlyIssue returns map of {reason->count} for normalization issues, with count=1 set only for the provided reason
func onlyIssue(reason string) map[string]int64 {
	m := noIssues()
	m[reason] = 1
	return m
}

func TestNormalizeOK(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeServicePassThru(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	before := s.Service
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, before, s.Service)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeEmptyServiceNoLang(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.Service = ""
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, s.Service, DefaultServiceName)
	assert.Equal(t, onlyIssue("service_empty"), statsToIssues(ts))
}

func TestNormalizeEmptyServiceWithLang(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.Service = ""
	ts.Lang = "java"
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, s.Service, ts.Lang)
	assert.Equal(t, onlyIssue("service_empty"), statsToIssues(ts))
}

func TestNormalizeLongService(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.Service = strings.Repeat("CAMEMBERT", 100)
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, s.Service, s.Service[:MaxServiceLen])
	assert.Equal(t, onlyIssue("service_truncate"), statsToIssues(ts))
}

func TestNormalizeNamePassThru(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	before := s.Name
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, before, s.Name)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeEmptyName(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.Name = ""
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, s.Name, DefaultSpanName)
	assert.Equal(t, onlyIssue("span_name_empty"), statsToIssues(ts))
}

func TestNormalizeLongName(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.Name = strings.Repeat("CAMEMBERT", 100)
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, s.Name, s.Name[:MaxNameLen])
	assert.Equal(t, onlyIssue("span_name_truncate"), statsToIssues(ts))
}

func TestNormalizeNameNoAlphanumeric(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.Name = "/"
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, s.Name, DefaultSpanName)
	assert.Equal(t, onlyIssue("span_name_invalid"), statsToIssues(ts))
}

func TestNormalizeNameForMetrics(t *testing.T) {
	expNames := map[string]string{
		"pylons.controller": "pylons.controller",
		"trace-api.request": "trace_api.request",
	}

	ts := &info.TagStats{}
	s := newTestSpan()
	for name, expName := range expNames {
		s.Name = name
		assert.NoError(t, normalize(ts, s))
		assert.Equal(t, expName, s.Name)
		assert.Equal(t, noIssues(), statsToIssues(ts))
	}
}

func TestNormalizeResourcePassThru(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	before := s.Resource
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, before, s.Resource)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeEmptyResource(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.Resource = ""
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, s.Resource, s.Name)
	assert.Equal(t, onlyIssue("resource_empty"), statsToIssues(ts))
}

func TestNormalizeTraceIDPassThru(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	before := s.TraceID
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, before, s.TraceID)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeNoTraceID(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.TraceID = 0
	assert.Error(t, normalize(ts, s))
	assert.Equal(t, onlyIssue("trace_id_zero"), statsToIssues(ts))
}

func TestNormalizeSpanIDPassThru(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	before := s.SpanID
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, before, s.SpanID)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeNoSpanID(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.SpanID = 0
	assert.Error(t, normalize(ts, s))
	assert.Equal(t, onlyIssue("span_id_zero"), statsToIssues(ts))
}

func TestNormalizeStartPassThru(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	before := s.Start
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, before, s.Start)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeStartTooSmall(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.Start = 42
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, onlyIssue("invalid_start_date"), statsToIssues(ts))

}

func TestNormalizeDurationPassThru(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	before := s.Duration
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, before, s.Duration)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeEmptyDuration(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.Duration = 0
	assert.NoError(t, normalize(ts, s))
	assert.EqualValues(t, s.Duration, 0)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeNegativeDuration(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.Duration = -50
	assert.NoError(t, normalize(ts, s))
	assert.EqualValues(t, s.Duration, 0)
	assert.Equal(t, onlyIssue("invalid_duration"), statsToIssues(ts))
}

func TestNormalizeErrorPassThru(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	before := s.Error
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, before, s.Error)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeMetricsPassThru(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	before := s.Metrics
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, before, s.Metrics)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeMetaPassThru(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	before := s.Meta
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, before, s.Meta)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeParentIDPassThru(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	before := s.ParentID
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, before, s.ParentID)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeTypePassThru(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	before := s.Type
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, before, s.Type)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeTypeTooLong(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.Type = strings.Repeat("sql", 1000)
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, onlyIssue("type_truncate"), statsToIssues(ts))
}

func TestNormalizeServiceTag(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.Service = "retargeting(api-Staging "
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, "retargeting_api-staging", s.Service)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeEnv(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.Meta["env"] = "DEVELOPMENT"
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, "development", s.Meta["env"])
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestSpecialZipkinRootSpan(t *testing.T) {
	ts := &info.TagStats{}
	s := newTestSpan()
	s.ParentID = 42
	s.TraceID = 42
	s.SpanID = 42
	beforeTraceID := s.TraceID
	beforeSpanID := s.SpanID
	assert.NoError(t, normalize(ts, s))
	assert.Equal(t, uint64(0), s.ParentID)
	assert.Equal(t, beforeTraceID, s.TraceID)
	assert.Equal(t, beforeSpanID, s.SpanID)
	assert.Equal(t, noIssues(), statsToIssues(ts))
}

func TestNormalizeTraceEmpty(t *testing.T) {
	ts, trace := &info.TagStats{}, pb.Trace{}
	err := normalizeTrace(ts, trace)
	assert.Error(t, err)
	assert.Equal(t, onlyIssue("empty_trace"), statsToIssues(ts))
}

func TestNormalizeTraceTraceIdMismatch(t *testing.T) {
	ts := &info.TagStats{}
	span1, span2 := newTestSpan(), newTestSpan()

	span1.TraceID = 1
	span2.TraceID = 2
	trace := pb.Trace{span1, span2}
	err := normalizeTrace(ts, trace)
	assert.Error(t, err)
	assert.Equal(t, onlyIssue("foreign_span"), statsToIssues(ts))
}

func TestNormalizeTraceInvalidSpan(t *testing.T) {
	ts := &info.TagStats{}
	span1, span2 := newTestSpan(), newTestSpan()

	span2.Name = "" // invalid
	trace := pb.Trace{span1, span2}
	err := normalizeTrace(ts, trace)
	assert.NoError(t, err)
	assert.Equal(t, onlyIssue("span_name_empty"), statsToIssues(ts))
}

func TestNormalizeTraceDuplicateSpanID(t *testing.T) {
	ts := &info.TagStats{}
	span1, span2 := newTestSpan(), newTestSpan()

	span2.SpanID = span1.SpanID
	trace := pb.Trace{span1, span2}
	err := normalizeTrace(ts, trace)
	assert.NoError(t, err)
	assert.Equal(t, onlyIssue("duplicate_span_id"), statsToIssues(ts))
}

func TestNormalizeTrace(t *testing.T) {
	ts := &info.TagStats{}
	span1, span2 := newTestSpan(), newTestSpan()

	span2.SpanID++
	trace := pb.Trace{span1, span2}
	err := normalizeTrace(ts, trace)
	assert.NoError(t, err)
}

func TestIsValidStatusCode(t *testing.T) {
	assert := assert.New(t)
	assert.True(isValidStatusCode("100"))
	assert.True(isValidStatusCode("599"))
	assert.False(isValidStatusCode("99"))
	assert.False(isValidStatusCode("600"))
	assert.False(isValidStatusCode("Invalid status code"))
}

func TestNormalizeInvalidUTF8(t *testing.T) {
	invalidUTF8 := "test\x99\x8f"

	t.Run("service", func(t *testing.T) {
		assert := assert.New(t)

		ts := &info.TagStats{}
		span := newTestSpan()

		span.Service = invalidUTF8

		err := normalize(ts, span)

		assert.Nil(err)
		assert.Equal("test", span.Service)
	})

	t.Run("resource", func(t *testing.T) {
		assert := assert.New(t)

		ts := &info.TagStats{}
		span := newTestSpan()

		span.Resource = invalidUTF8

		err := normalize(ts, span)

		assert.Nil(err)
		assert.Equal("test��", span.Resource)
	})

	t.Run("name", func(t *testing.T) {
		assert := assert.New(t)

		ts := &info.TagStats{}
		span := newTestSpan()

		span.Name = invalidUTF8

		err := normalize(ts, span)

		assert.Nil(err)
		assert.Equal("test", span.Name)
	})

	t.Run("type", func(t *testing.T) {
		assert := assert.New(t)

		ts := &info.TagStats{}
		span := newTestSpan()

		span.Type = invalidUTF8

		err := normalize(ts, span)

		assert.Nil(err)
		assert.Equal("test��", span.Type)
	})

	t.Run("meta", func(t *testing.T) {
		assert := assert.New(t)

		ts := &info.TagStats{}
		span := newTestSpan()

		span.Meta = map[string]string{
			invalidUTF8: "test1",
			"test2":     invalidUTF8,
		}

		err := normalize(ts, span)

		assert.Nil(err)
		assert.EqualValues(map[string]string{
			"test��": "test1",
			"test2":  "test��",
		}, span.Meta)
	})
}

func BenchmarkNormalization(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ts := &info.TagStats{}
		span := newTestSpan()

		normalize(ts, span)
	}
}

func TestNormalizeTag(t *testing.T) {
	for _, tt := range []struct{ in, out string }{
		{in: "#test_starting_hash", out: "test_starting_hash"},
		{in: "TestCAPSandSuch", out: "testcapsandsuch"},
		{in: "Test Conversion Of Weird !@#$%^&**() Characters", out: "test_conversion_of_weird_characters"},
		{in: "$#weird_starting", out: "weird_starting"},
		{in: "allowed:c0l0ns", out: "allowed:c0l0ns"},
		{in: "1love", out: "love"},
		{in: "ünicöde", out: "ünicöde"},
		{in: "ünicöde:metäl", out: "ünicöde:metäl"},
		{in: "Data🐨dog🐶 繋がっ⛰てて", out: "data_dog_繋がっ_てて"},
		{in: " spaces   ", out: "spaces"},
		{in: " #hashtag!@#spaces #__<>#  ", out: "hashtag_spaces"},
		{in: ":testing", out: ":testing"},
		{in: "_foo", out: "foo"},
		{in: ":::test", out: ":::test"},
		{in: "contiguous_____underscores", out: "contiguous_underscores"},
		{in: "foo_", out: "foo"},
		{in: "\u017Fodd_\u017Fcase\u017F", out: "\u017Fodd_\u017Fcase\u017F"}, // edge-case
		{in: "", out: ""},
		{in: " ", out: ""},
		{in: "ok", out: "ok"},
		{in: "™Ö™Ö™™Ö™", out: "ö_ö_ö"},
		{in: "AlsO:ök", out: "also:ök"},
		{in: ":still_ok", out: ":still_ok"},
		{in: "___trim", out: "trim"},
		{in: "12.:trim@", out: ":trim"},
		{in: "12.:trim@@", out: ":trim"},
		{in: "fun:ky__tag/1", out: "fun:ky_tag/1"},
		{in: "fun:ky@tag/2", out: "fun:ky_tag/2"},
		{in: "fun:ky@@@tag/3", out: "fun:ky_tag/3"},
		{in: "tag:1/2.3", out: "tag:1/2.3"},
		{in: "---fun:k####y_ta@#g/1_@@#", out: "fun:k_y_ta_g/1"},
		{in: "AlsO:œ#@ö))œk", out: "also:œ_ö_œk"},
		{in: "test\x99\x8faaa", out: "test_aaa"},
		{in: "test\x99\x8f", out: "test"},
		{in: strings.Repeat("a", 888), out: strings.Repeat("a", 200)},
		{
			in: func() string {
				b := bytes.NewBufferString("a")
				for i := 0; i < 799; i++ {
					_, err := b.WriteRune('🐶')
					assert.NoError(t, err)
				}
				_, err := b.WriteRune('b')
				assert.NoError(t, err)
				return b.String()
			}(),
			out: "a", // 'b' should have been truncated
		},
		{"a" + string(unicode.ReplacementChar), "a"},
		{"a" + string(unicode.ReplacementChar) + string(unicode.ReplacementChar), "a"},
		{"a" + string(unicode.ReplacementChar) + string(unicode.ReplacementChar) + "b", "a_b"},
	} {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.out, normalizeTag(tt.in), tt.in)
		})
	}
}

func benchNormalizeTag(tag string) func(b *testing.B) {
	return func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			normalizeTag(tag)
		}
	}
}

func BenchmarkNormalizeTag(b *testing.B) {
	b.Run("ok", benchNormalizeTag("good_tag"))
	b.Run("trim", benchNormalizeTag("___trim_left"))
	b.Run("trim-both", benchNormalizeTag("___trim_right@@#!"))
	b.Run("plenty", benchNormalizeTag("fun:ky_ta@#g/1"))
	b.Run("more", benchNormalizeTag("fun:k####y_ta@#g/1_@@#"))
}
