// Package cronexpr 实现 Quartz 风格的 cron 表达式解析与下次时间计算，
// 与 xxl-job-admin 内置的 CronExpression（自 Quartz 移植）语义对齐：
// 支持 6/7 字段（秒 分 时 日 月 周 [年]）、名称(JAN-DEC/SUN-SAT)、
// L、W、LW、n#m、nL 等扩展语法。
package cronexpr

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

var monthNames = map[string]int{"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6, "JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12}
var dowNames = map[string]int{"SUN": 1, "MON": 2, "TUE": 3, "WED": 4, "THU": 5, "FRI": 6, "SAT": 7}

type domFn func(y int, m time.Month, d int) bool
type dowFn func(y int, m time.Month, wd time.Weekday) bool

type Expression struct {
	seconds []int // 升序去重

	minutes [60]bool
	hours   [24]bool
	months  [13]bool
	years   map[int]bool

	domNums       [32]bool
	domRestricted bool
	domFns        []domFn

	dowNums       [8]bool
	dowRestricted bool
	dowFns        []dowFn
}

// Parse 解析 cron 表达式，失败返回 error。
func Parse(expr string) (*Expression, error) {
	fields := strings.Fields(expr)
	if len(fields) != 6 && len(fields) != 7 {
		return nil, fmt.Errorf("cron 表达式字段数错误(需6或7个): %q", expr)
	}
	e := &Expression{years: map[int]bool{}}

	secSet, err := parseNumSet(fields[0], 0, 59, nil)
	if err != nil {
		return nil, fmt.Errorf("秒字段错误: %w", err)
	}
	e.seconds = secSet

	minSet, err := parseNumSet(fields[1], 0, 59, nil)
	if err != nil {
		return nil, fmt.Errorf("分字段错误: %w", err)
	}
	fillBool(e.minutes[:], minSet)

	hourSet, err := parseNumSet(fields[2], 0, 23, nil)
	if err != nil {
		return nil, fmt.Errorf("时字段错误: %w", err)
	}
	fillBool(e.hours[:], hourSet)

	monSet, err := parseNumSet(fields[4], 1, 12, monthNames)
	if err != nil {
		return nil, fmt.Errorf("月字段错误: %w", err)
	}
	fillBool(e.months[:], monSet)

	yearSet := ([]int)(nil)
	if len(fields) == 7 {
		yearSet, err = parseNumSet(fields[6], 1970, 2999, nil)
		if err != nil {
			return nil, fmt.Errorf("年字段错误: %w", err)
		}
	}
	if yearSet == nil {
		e.years = nil // 未指定年份(或*) = 不限制
	} else {
		for _, v := range yearSet {
			e.years[v] = true
		}
	}

	if err := e.parseDom(fields[3]); err != nil {
		return nil, err
	}
	if err := e.parseDow(fields[5]); err != nil {
		return nil, err
	}
	return e, nil
}

// parseDom 解析 day-of-month 字段：支持 ? * , - / 数字 L W nW LW
func (e *Expression) parseDom(field string) error {
	if field == "?" {
		return nil
	}
	for _, token := range strings.Split(field, ",") {
		switch {
		case token == "*" || token == "":
			// 全范围
			for d := 1; d <= 31; d++ {
				e.domNums[d] = true
			}
		case token == "L":
			e.domFns = append(e.domFns, func(y int, m time.Month, d int) bool {
				return d == lastDayOfMonth(y, m)
			})
		case token == "LW":
			e.domFns = append(e.domFns, func(y int, m time.Month, d int) bool {
				return d == lastWeekdayOfMonth(y, m)
			})
		case strings.HasSuffix(token, "W"):
			n, err := strconv.Atoi(strings.TrimSuffix(token, "W"))
			if err != nil || n < 1 || n > 31 {
				return fmt.Errorf("日字段 W 语法错误: %q", token)
			}
			e.domFns = append(e.domFns, func(y int, m time.Month, d int) bool {
				return d == nearestWeekday(y, m, n)
			})
		default:
			nums, err := parseNumSet(token, 1, 31, nil)
			if err != nil {
				return fmt.Errorf("日字段错误: %w", err)
			}
			for _, v := range nums {
				e.domNums[v] = true
			}
		}
	}
	e.domRestricted = e.isDomRestricted()
	return nil
}

func (e *Expression) isDomRestricted() bool {
	count := 0
	for d := 1; d <= 31; d++ {
		if e.domNums[d] {
			count++
		}
	}
	if count < 31 || len(e.domFns) > 0 {
		return true
	}
	return false
}

// parseDow 解析 day-of-week 字段：支持 ? * , - / 数字 名称 L nL n#m
func (e *Expression) parseDow(field string) error {
	if field == "?" {
		return nil
	}
	for _, token := range strings.Split(field, ",") {
		switch {
		case token == "*" || token == "":
			for d := 1; d <= 7; d++ {
				e.dowNums[d] = true
			}
		case token == "L":
			e.dowNums[7] = true // L 单独 = 7(SAT)
		case strings.HasSuffix(token, "L"):
			n, err := parseDowValue(strings.TrimSuffix(token, "L"))
			if err != nil || n < 1 || n > 7 {
				return fmt.Errorf("周字段 nL 语法错误: %q", token)
			}
			e.dowFns = append(e.dowFns, func(y int, m time.Month, wd time.Weekday) bool {
				return int(wd)+1 == n && isLastWeekdayOfMonthOccur(y, m, wd)
			})
		case strings.Contains(token, "#"):
			parts := strings.Split(token, "#")
			if len(parts) != 2 {
				return fmt.Errorf("周字段 # 语法错误: %q", token)
			}
			n, err1 := parseDowValue(parts[0])
			k, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil || n < 1 || n > 7 || k < 1 || k > 5 {
				return fmt.Errorf("周字段 # 语法错误: %q", token)
			}
			e.dowFns = append(e.dowFns, func(y int, m time.Month, wd time.Weekday) bool {
				if int(wd)+1 != n {
					return false
				}
				first := time.Date(y, m, 1, 0, 0, 0, 0, time.Local)
				day := 1 + (n-int(first.Weekday())-1+14)%7 + (k-1)*7
				return day <= lastDayOfMonth(y, m)
			})
		default:
			nums, err := parseNumSet(token, 1, 7, dowNames)
			if err != nil {
				return fmt.Errorf("周字段错误: %w", err)
			}
			for _, v := range nums {
				e.dowNums[v] = true
			}
		}
	}
	count := 0
	for d := 1; d <= 7; d++ {
		if e.dowNums[d] {
			count++
		}
	}
	e.dowRestricted = count < 7 || len(e.dowFns) > 0
	return nil
}

func parseDowValue(s string) (int, error) {
	if v, ok := dowNames[strings.ToUpper(s)]; ok {
		return v, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// parseNumSet 解析数值集合字段：* / a a-b a-b/s a/s /s，返回 nil 表示"全部"。
func parseNumSet(field string, min, max int, names map[string]int) ([]int, error) {
	var result []int
	full := false
	for _, token := range strings.Split(field, ",") {
		step := 1
		start, end := min, max
		body := token
		if i := strings.Index(token, "/"); i >= 0 {
			var err error
			step, err = strconv.Atoi(token[i+1:])
			if err != nil || step < 1 {
				return nil, fmt.Errorf("步长语法错误: %q", token)
			}
			body = token[:i]
		}
		switch {
		case body == "*" || body == "" || body == "?":
			start, end = min, max
		case strings.Contains(body, "-"):
			parts := strings.SplitN(body, "-", 2)
			var err error
			start, err = parseWithNames(parts[0], names)
			if err != nil {
				return nil, err
			}
			end, err = parseWithNames(parts[1], names)
			if err != nil {
				return nil, err
			}
		default:
			var err error
			start, err = parseWithNames(body, names)
			if err != nil {
				return nil, err
			}
			end = max // a/s = 从 a 到 max
			if step == 1 {
				end = start // 纯数字
			}
		}
		if start < min || end > max || start > end {
			return nil, fmt.Errorf("数值越界(%d-%d): %q", min, max, token)
		}
		if start == min && end == max && step == 1 {
			full = true
		}
		for v := start; v <= end; v += step {
			result = append(result, v)
		}
	}
	if full {
		return nil, nil
	}
	sort.Ints(result)
	// 去重
	uniq := result[:0]
	for i, v := range result {
		if i == 0 || result[i-1] != v {
			uniq = append(uniq, v)
		}
	}
	return uniq, nil
}

func parseWithNames(s string, names map[string]int) (int, error) {
	if names != nil {
		if v, ok := names[strings.ToUpper(s)]; ok {
			return v, nil
		}
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("非法数值: %q", s)
	}
	return v, nil
}

// fillBool 布尔位图填充：nums 为 nil 表示全选。
func fillBool(dst []bool, nums []int) {
	if nums == nil {
		for i := range dst {
			dst[i] = true
		}
		return
	}
	for _, v := range nums {
		if v < len(dst) {
			dst[v] = true
		}
	}
}

func lastDayOfMonth(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.Local).Day()
}

func weekdayOf(y int, m time.Month, d int) time.Weekday {
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local).Weekday()
}

// nearestWeekday 计算与当月 n 号最近的工作日（Quartz W 语义）。
func nearestWeekday(y int, m time.Month, n int) int {
	if n > lastDayOfMonth(y, m) {
		return -1
	}
	switch weekdayOf(y, m, n) {
	case time.Saturday:
		if n-1 >= 1 {
			return n - 1
		}
		return n + 2
	case time.Sunday:
		if n+1 <= lastDayOfMonth(y, m) {
			return n + 1
		}
		return n - 2
	default:
		return n
	}
}

// lastWeekdayOfMonth 计算当月最后一个工作日（LW 语义）。
func lastWeekdayOfMonth(y int, m time.Month) int {
	last := lastDayOfMonth(y, m)
	switch weekdayOf(y, m, last) {
	case time.Saturday:
		return last - 1
	case time.Sunday:
		return last - 2
	default:
		return last
	}
}

func isLastWeekdayOfMonthOccur(y int, m time.Month, wd time.Weekday) bool {
	last := lastDayOfMonth(y, m)
	diff := int(weekdayOf(y, m, last)) - int(wd)
	return diff >= 0 && diff < 7
}

func (e *Expression) matchDay(y int, m time.Month, d int) bool {
	wd := weekdayOf(y, m, d)
	domMatch := e.domNums[d]
	for _, fn := range e.domFns {
		if fn(y, m, d) {
			domMatch = true
			break
		}
	}
	dowMatch := e.dowNums[int(wd)+1]
	for _, fn := range e.dowFns {
		if fn(y, m, wd) {
			dowMatch = true
			break
		}
	}
	// Quartz 规则：日与周都受限时，任一命中即匹配
	if e.domRestricted && e.dowRestricted {
		return domMatch || dowMatch
	}
	if e.domRestricted {
		return domMatch
	}
	if e.dowRestricted {
		return dowMatch
	}
	return true
}

// Next 返回 t 之后的下一次触发时间；4 年内无匹配返回零值。
func (e *Expression) Next(t time.Time) time.Time {
	limit := t.AddDate(4, 0, 0)
	start := t.Add(time.Second)
	cur := start.Truncate(time.Minute)
	startSec := start.Second()

	for cur.Before(limit) {
		y, m, d := cur.Date()
		if (e.years == nil || e.years[y]) && e.months[int(m)] && e.matchDay(y, m, d) &&
			e.hours[cur.Hour()] && e.minutes[cur.Minute()] {
			for _, s := range e.seconds {
				if s < startSec {
					continue
				}
				return time.Date(y, m, d, cur.Hour(), cur.Minute(), s, 0, cur.Location())
			}
		}
		cur = cur.Add(time.Minute)
		startSec = 0
	}
	return time.Time{}
}

// NextN 返回 from 之后 n 个触发时间（不足时返回已找到的）。
func (e *Expression) NextN(from time.Time, n int) []time.Time {
	var result []time.Time
	t := from
	for len(result) < n {
		next := e.Next(t)
		if next.IsZero() {
			break
		}
		result = append(result, next)
		t = next
	}
	return result
}
