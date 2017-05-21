package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"
)

type ShiftTable map[string]Shift

type Shift struct {
	date  string
	table []string
}

const layout = "1/2"

func main() {

	app := cli.NewApp()
	st := NewShiftTable(os.Getenv("OFLS_KEY"), os.Getenv("OFLS_GID"))

	app.Name = "shien"
	app.Usage = "show time shift of OFLS"

	app.Commands = []cli.Command{
		{Name: "date",
			Aliases: []string{"d"},
			Usage:   "show date shift ex) $shien d 5/22; $shien d 3;",
			Action: func(c *cli.Context) error {
				fmt.Println(st.Day(c.Args().First()))
				return nil
			},
		},
		{Name: "week",
			Aliases: []string{"w"},
			Usage:   "show week shift ex) $shien w 5/22; $shien w 1;",
			Action: func(c *cli.Context) error {
				fmt.Println(st.Week(c.Args().First()))
				return nil
			},
		},
		{Name: "table",
			Aliases: []string{"t"},
			Usage:   "show week shift as a table ex) $shien t 5/22; $shien t 1;",
			Action: func(c *cli.Context) error {
				st.ShowWeekTable(c.Args().First())
				return nil
			},
		},
	}
	app.Action = func(c *cli.Context) error {
		fmt.Println(st.Today())
		return nil
	}
	app.Version = "0.0.1"

	app.Run(os.Args)
}

func (st ShiftTable) Today() string {
	return st.Day("0")
}

func (st ShiftTable) Day(daystr string) string {
	re1 := regexp.MustCompile(`\d+\/\d+`)
	re2 := regexp.MustCompile(`^-?\d+$`)
	switch {
	case re1.MatchString(daystr):
		return st[re1.FindString(daystr)].FormatTable()

	case re2.MatchString(daystr):
		i, err := strconv.Atoi(re2.FindString(daystr))
		if err != nil {
			panic(err)
		}

		return st.DayShift(i).FormatTable()
	case daystr == "":
		return st.Today()
	}

	return "invalid argument. format must be like 3/9 or integer."
}

func (st ShiftTable) DayShift(i int) Shift {
	day := time.Now()
	return st[(day.AddDate(0, 0, i)).Format(layout)]
}

func (st ShiftTable) ShowWeekTable(daystr string) {
	start, end := getShowRange(daystr)
	table := tablewriter.NewWriter(os.Stdout)

	table.SetHeader([]string{"Date", "1st", "2nd", "lunch",
		"3rd", "4th", "5th", "night"})

	for d := start; d < end; d++ {
		shift := st.DayShift(d)
		row := make([]string, 8)
		row[0] = shift.date
		for i := 0; i < 7; i++ {
			row[i+1] = shift.table[i]
		}

		table.Append(row)
	}

	table.Render()
}

func getShowRange(daystr string) (start, end int) {
	re1 := regexp.MustCompile(`\d+\/\d+`)
	re2 := regexp.MustCompile(`^-?\d+$`)
	now := time.Now()

	switch {
	case re1.MatchString(daystr):

		day, err := time.Parse(layout, re1.FindString(daystr))
		if err != nil {
			panic(err)
		}

		day = day.AddDate(now.Year(), 0, 0)
		diff := int(day.Sub(now).Hours() / 24)
		weekDay := int(day.Weekday() - 1)
		start = diff - weekDay
		end = start + 5

	case daystr == "":
		daystr = "0"
		fallthrough

	case re2.MatchString(daystr):

		week, err := strconv.Atoi(re2.FindString(daystr))
		if err != nil {
			panic(err)
		}

		weekDay := int(now.Weekday()) - 1
		start = week*7 - weekDay
		end = start + 5
	}

	return
}

func (st ShiftTable) Week(daystr string) string {

	start, end := getShowRange(daystr)

	var buf bytes.Buffer
	for d := start; d < end; d++ {
		buf.WriteString(st.Day(strconv.Itoa(d)))
		if d != end-1 {
			buf.WriteString("\n\n")
		}
	}

	return buf.String()
}

func NewShiftTable(key string, gid string) ShiftTable {
	url := "https://docs.google.com/spreadsheets/d/" + key + "/export?format=csv&gid=" + gid
	response, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	reader := csv.NewReader(response.Body)
	return getShiftMap(reader)
}

func getShiftMap(r *csv.Reader) ShiftTable {

	re := regexp.MustCompile(`\d+\/\d+`)
	shiftmap := map[string]Shift{}

	for {
		record, err := r.Read()

		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Println("Read error: ", err)
			break
		}

		if re.MatchString(record[0]) {
			result := re.FindString(record[0])
			shiftmap[result] = Shift{record[0], getOneDayTable(record[1:])}
		}
	}

	return shiftmap
}

func getOneDayTable(record []string) []string {

	return []string{
		strings.TrimRight(strings.Join(record[0:1], "，"), "/,*$/"),
		strings.TrimRight(strings.Join(record[2:3], ","), "/,*$/"),
		strings.TrimRight(strings.Join(record[4:6], ","), "/,*$/"),
		strings.TrimRight(strings.Join(record[7:9], ","), "/,*$/"),
		strings.TrimRight(strings.Join(record[10:12], ","), "/,*$/"),
		strings.TrimRight(strings.Join(record[13:15], ","), "/,*$/"),
		strings.TrimRight(strings.Join(record[16:21], ","), "/,*$/"),
		record[22], record[23], record[24]}
}

func (shift Shift) FormatTable() string {

	indices := [10]string{
		"1st",
		"2nd",
		"lun",
		"3rd",
		"4th",
		"5th",
		"nig",
		"-----\nmur",
		"hig",
		"etc"}

	var buf bytes.Buffer

	buf.WriteString(shift.date)

	for i, names := range shift.table {
		buf.WriteString("\n")
		buf.WriteString(indices[i])
		buf.WriteString(" : ")
		buf.WriteString(names)
	}

	return buf.String()
}
