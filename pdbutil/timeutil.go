package pdbutil

import "time"

// ISOTimeFormat iso time format yyyy-mm-dd HH:MM:SS
const ISOTimeFormat = "2006-01-02 15:04:05"
const ISOTimeFormatzzz = "2006-01-02 15:04:05.000"

// NowTimeStrInLocal return yyyy-mm-dd hh:mm:ss in local time
func NowTimeStrInLocal() string {
	t := time.Now()
	return t.Format(ISOTimeFormat)
}

// NowTimeStrInUtc return yyyy-mm-dd hh:mm:ss in utc time
func NowTimeStrInUtc() string {
	t := time.Now().UTC()
	return t.Format(ISOTimeFormat)
}

// NowTimeStrInUtcZzz return yyyy-mm-dd hh:mm:ss.zzz in utc time
func NowTimeStrInUtcZzz() string {
	t := time.Now().UTC()
	return t.Format(ISOTimeFormatzzz)
}

func TimeISOStr(t time.Time) string {
	return t.Format(ISOTimeFormat)
}

func GetUnixEpochInMilliseconds(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}

func GetNowUnixEpochInMilliseconds() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// get utc time format yyyy-mm-dd HH:MM:SS of time
func GetUtcTimeStr(t time.Time) string {
	return t.UTC().Format(ISOTimeFormat)
}

// get utc time format yyyy-mm-dd HH:MM:SS of time
func GetUtcTimeStrzzz(t time.Time) string {
	return t.UTC().Format(ISOTimeFormatzzz)
}

// ParseUTCTime parse yyyy-mm-dd HH:MM:SS as utc time
// when parse err, return a empty time.Time
func ParseUTCTime(timeStr string) time.Time {
	result, err := time.Parse(ISOTimeFormat, timeStr)
	if err == nil {
		return result
	} else {
		return time.Time{}
	}
}
