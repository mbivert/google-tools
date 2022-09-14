package main

import (
	"log"
	"fmt"
	"time"
	"os"
	"context"
	"strings"
	"google.golang.org/api/searchconsole/v1"
	"google.golang.org/api/option"
)

/************************************************************
 * Queries
 */

func getSites(scs *searchconsole.Service) ([]string, error) {
	xs := []string{}
	slr, err := scs.Sites.List().Do()
	if err != nil {
		return xs, err
	}
	for _, e := range slr.SiteEntry {
		xs = append(xs, e.SiteUrl)
	}

	return xs, nil
}

func getSitemaps(scs *searchconsole.Service, s string) ([]string, error) {
	xs := []string{}
	slr, err := scs.Sitemaps.List(s).Do()
	if err != nil {
		return xs, nil
	}
	for _, e := range slr.Sitemap {
		xs = append(xs, e.Path)
	}
	return xs, nil
}

func queryAnalytics(scs *searchconsole.Service, s string, dims []string, start, end string) ([]*searchconsole.ApiDataRow, error) {
	args := searchconsole.SearchAnalyticsQueryRequest{
		Dimensions : dims,
		StartDate  : start,
		EndDate    : end,
		DataState  : "ALL",
	}

	saqr, err := scs.Searchanalytics.Query(s, &args).Do()
	if err != nil {
		return nil, err
	}
	return saqr.Rows, nil
}

/************************************************************
 * Aux
 */

// YYYY-MM-DD format for time.Parse()/.Format()
var YYYYMMDD = "2006-01-02"

// last day for which we had clicks
func getLastDay(xs []*searchconsole.ApiDataRow) (string, error) {
	n := time.Now().UTC().AddDate(-20, 0, 0)

	for _, x := range xs {
		if x.Clicks == 0 {
			continue
		}
		if len(x.Keys) == 0 {
			return "", fmt.Errorf("Empty Keys")
		}
		d, err := time.Parse(YYYYMMDD , x.Keys[0])
		if err != nil {
			return "", fmt.Errorf("First Keys element is not an YYYY-MM-DD: %s", x.Keys[0])
		}

		if d.After(n) {
			n = d
		}
	}
	return n.Format(YYYYMMDD), nil
}

/************************************************************
 * Main commands
 */

func lsSites(scs *searchconsole.Service) error {
	xs, err := getSites(scs)
	if err != nil {
		return err
	}
	for _, x := range xs {
		fmt.Println(x)
	}
	return nil
}

func lsSitemaps(scs *searchconsole.Service, s string) error {
	xs, err := getSitemaps(scs, s)
	if err != nil {
		return err
	}
	for _, x := range xs {
		fmt.Println(x)
	}
	return nil
}

func printHeader(xs []*searchconsole.ApiDataRow, last string) {
	nc := 0
	ni := 0
	for _, x := range xs {
		// NOTE: always there, cf. getLastDay()
		if last != "" && x.Keys[0] != last {
			continue
		}
		nc += int(x.Clicks)
		ni += int(x.Impressions)
	}
	// NOTE: averaging the x.Ctr yields a result discordant
	// with what's displayed on the Web Console; likely
	// an accumulation of rounding errors. This is much closer:
	ct := float64(nc)/float64(ni)
	fmt.Printf("----------------------------------\n")
	fmt.Printf("%-10s %-10s %-5s %s\n", "Clicks", "Impr.", "Ctr.", "Pages")
	fmt.Printf("%-10d %-10d %-5.2f %s\n", nc, ni, ct*100, "Total")
	fmt.Printf("----------------------------------\n")
}

func queryAllAnalytics(scs *searchconsole.Service, s string, full bool) error {
	n := time.Now().UTC()
	// 20 years ago
	xs, err := queryAnalytics(
		scs, s, []string{"PAGE"},
		n.AddDate(-20, 0, 0).Format(YYYYMMDD),
		n.Format(YYYYMMDD))
	if err != nil {
		return err
	}
	printHeader(xs, "")
	for _, x := range xs {
		if !full && x.Clicks == 0. {
			break
		}
		fmt.Printf("%-10d %-10d %-5.2f %s\n", int(x.Clicks), int(x.Impressions), x.Ctr*100, strings.Join(x.Keys, ", "))
	}
	return nil
}

func queryLastAnalytics(scs *searchconsole.Service, s string) error {
	n := time.Now().UTC()

	// 10 days ago should be good enough; stats are usually
	// updated in less than a day, around 2d before today
	xs, err := queryAnalytics(
		scs, s, []string{"DATE", "PAGE"},
		n.AddDate(0, 0, -10).Format(YYYYMMDD),
		n.Format(YYYYMMDD))
	if err != nil {
		return err
	}

	last, err := getLastDay(xs)
	if err != nil {
		return err
	}
	fmt.Printf("----------------------------------\n")
	fmt.Printf("Last day: %s\n", last)
	printHeader(xs, last)
	for _, x := range xs {
		// NOTE: always there, cf. getLastDay()
		if x.Keys[0] != last {
			continue
		}
		fmt.Printf("%-10d %-10d %-5.2f %s\n", int(x.Clicks), int(x.Impressions), x.Ctr*100, x.Keys[1])
	}

	return nil
}

func queryDayAnalytics(scs *searchconsole.Service, s, d string) error {
	xs, err := queryAnalytics(
		scs, s, []string{"DATE", "PAGE"}, d, d,
	)
	if err != nil {
		return err
	}

	fmt.Printf("----------------------------------\n")
	fmt.Printf("Day: %s\n", d)
	printHeader(xs, d)
	for _, x := range xs {
		fmt.Printf("%-10d %-10d %-5.2f %s\n", int(x.Clicks), int(x.Impressions), x.Ctr*100, x.Keys[1])
	}

	return nil
}

func help(n int) {
	fmt.Println("TODO")
	os.Exit(n)
}

func main() {
	var scs *searchconsole.Service
	var err error

	if len(os.Args) < 2 {
		help(1)
	}

	ctx := context.Background()

	n := 1
	if os.Args[1] == "-c" {
		// args0 -c <file> <cmd>
		if len(os.Args) < 4 {
			help(1)
		}
		scs, err = searchconsole.NewService(ctx, option.WithCredentialsFile(os.Args[2]))
		n = 3
	} else {
		// Will rely on e.g. environ's GOOGLE_APPLICATION_CREDENTIALS
		scs, err = searchconsole.NewService(ctx)
	}

	if err != nil {
		log.Fatal(err)
	}

	switch os.Args[n] {
	case "ls-sites":
		if err := lsSites(scs); err != nil {
			log.Fatal(err)
		}
	case "ls-sitemaps":
		if len(os.Args) < n {
			help(1)
		}
		if err := lsSitemaps(scs, os.Args[n+1]); err != nil {
			log.Fatal(err)
		}
	case "query-all":
		if len(os.Args) < n+2 {
			help(1)
		}
		if err := queryAllAnalytics(scs, os.Args[n+1], false); err != nil {
			log.Fatal(err)
		}
	case "query-last":
		if len(os.Args) < n+2 {
			help(1)
		}
		if err := queryLastAnalytics(scs, os.Args[n+1]); err != nil {
			log.Fatal(err)
		}
	case "query-day":
		if len(os.Args) < n+3 {
			help(1)
		}
		// TODO: -1 : yesterday, -2: two days ago, etc.
		if err := queryDayAnalytics(scs, os.Args[n+1], os.Args[n+2]); err != nil {
			log.Fatal(err)
		}
	case "query-full":
		if len(os.Args) < n+2 {
			help(1)
		}
		if err := queryAllAnalytics(scs, os.Args[n+1], true); err != nil {
			log.Fatal(err)
		}
	case "help":
		help(0)
	default:
		help(1)
	}
}