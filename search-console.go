package main

import (
	"log"
	"fmt"
	"time"
	"os"
	"context"
	"strings"
	"strconv"
	"path/filepath"
	"errors"
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

func mkSite(s string) string {
	if !strings.HasPrefix(s, "https://") && !strings.HasPrefix(s, "http://") {
		s = "https://"+s
	}
	return s
}

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

func stripSite(x, s string) string {
	// TrimRight: removes all trailings / from site name
	// e.g. https://foo/// -> https://foo
	return strings.TrimPrefix(x, strings.TrimRight(s, "/"))
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

func printHeader(xs []*searchconsole.ApiDataRow, last, cn string) {
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
	fmt.Printf("%-10s %-10s %-5s %s\n", "Clicks", "Impr.", "Ctr.", cn)
	fmt.Printf("%-10d %-10d %-5.2f %s\n", nc, ni, ct*100, "Total")
	fmt.Printf("----------------------------------\n")
}

func querySinceAnalytics(scs *searchconsole.Service, s, from string, full bool) error {
	// 20 years ago
	xs, err := queryAnalytics(
		scs, s, []string{"PAGE"},
		from,
		time.Now().UTC().Format(YYYYMMDD))
	if err != nil {
		return err
	}
	printHeader(xs, "", "Pages")
	for _, x := range xs {
		if !full && x.Clicks == 0. {
			break
		}
		fmt.Printf(
			"%-10d %-10d %-5.2f %s\n",
			int(x.Clicks),
			int(x.Impressions),
			x.Ctr*100,
			stripSite(x.Keys[0], s),
		)
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
	printHeader(xs, last, "Pages")
	for _, x := range xs {
		// NOTE: always there, cf. getLastDay()
		if x.Keys[0] != last {
			continue
		}
		fmt.Printf(
			"%-10d %-10d %-5.2f %s\n",
			int(x.Clicks),
			int(x.Impressions),
			x.Ctr*100,
			stripSite(x.Keys[1], s),
		)
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
	printHeader(xs, d, "Pages")
	for _, x := range xs {
		fmt.Printf(
			"%-10d %-10d %-5.2f %s\n",
			int(x.Clicks),
			int(x.Impressions),
			x.Ctr*100,
			stripSite(x.Keys[1], s),
		)
	}

	return nil
}

func queryKeywordsFull(scs *searchconsole.Service, s, p string) error {
	n := time.Now().UTC()

	// Flexible input
	s = strings.TrimRight(s, "/")
	if p == "" || p[0] != '/' {
		p = "/" + p
	}

	args := searchconsole.SearchAnalyticsQueryRequest{
		Dimensions : []string{"QUERY"},
		DimensionFilterGroups : []*searchconsole.ApiDimensionFilterGroup{
			&searchconsole.ApiDimensionFilterGroup{
				Filters : []*searchconsole.ApiDimensionFilter{
					&searchconsole.ApiDimensionFilter {
						Dimension  : "page",
						Operator   : "equals",
						Expression : s+p,
					},
				},
			},
		},
		StartDate  : n.AddDate(-20, 0, 0).Format(YYYYMMDD),
		EndDate    : n.Format(YYYYMMDD),
		DataState  : "ALL",
	}

	saqr, err := scs.Searchanalytics.Query(s, &args).Do()
	if err != nil {
		return err
	}

	xs := saqr.Rows
	fmt.Printf("----------------------------------\n")
	fmt.Printf("Page: %s\n", p)
	printHeader(xs, "", "Keywords")
	for _, x := range xs {
		fmt.Printf(
			"%-10d %-10d %-5.2f %s\n",
			int(x.Clicks),
			int(x.Impressions),
			x.Ctr*100,
			x.Keys[0],
		)
	}

	return nil
}

func help(n int) {
	fmt.Println("TODO")
	os.Exit(n)
}

// -n : today - n days; otherwise assume we
// already have a YYYY-MM-DD and returns it
func parseDate(s string) (string, error) {
	// -n : today - n days
	if s[0] == '-' {
		n, err := strconv.Atoi(s)
		if err != nil {
			return "", fmt.Errorf("Invalid date shortcut: %s", s)
		}
		s = time.Now().UTC().AddDate(0, 0, n).Format(YYYYMMDD)
	}
	return s, nil
}

func main() {
	var scs *searchconsole.Service
	var err error

	if len(os.Args) < 2 {
		help(1)
	}

	ctx := context.Background()
	n := 1

	// TODO: clumsy os.Args parsing
	var doQuerySinceAnalytics = func(full bool) {
		if len(os.Args) < n+2 {
			help(1)
		}
		d := time.Now().UTC().AddDate(-20, 0, 0).Format(YYYYMMDD)
		if len(os.Args) == n+3 {
			d = os.Args[n+2]
		}
		d, err := parseDate(d)
		if err != nil {
			log.Fatal(err)
		}
		if err := querySinceAnalytics(scs, mkSite(os.Args[n+1]), d, full); err != nil {
			log.Fatal(err)
		}
	}

	if os.Args[1] == "-c" {
		// args0 -c <file> <cmd>
		if len(os.Args) < 4 {
			help(1)
		}
		scs, err = searchconsole.NewService(ctx, option.WithCredentialsFile(os.Args[2]))
		n = 3
	} else {
		fn := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
		if fn != "" {
			// XXX/TODO: there's a pointer referencing error occuring
			// when GOOGLE_APPLICATION_CREDENTIALS refers to an inexisting
			// file; haven't checked what happens with an invalid file.
			//
			// Note also that if we call searchconsole.NewService(ctx),
			// relying on GOOGLE_APPLICATION_CREDENTIALS, and
			// GOOGLE_APPLICATION_CREDENTIALS isn't set, we would have
			// the same pointer error.
			if _, err := os.Stat(fn); errors.Is(err, os.ErrNotExist) {
				log.Fatal(fn, " does not exists")
			}
			// Implicitly use GOOGLE_APPLICATION_CREDENTIALS
			scs, err = searchconsole.NewService(ctx)
		} else {
			// If there's a search-console*.json file in current directory, use
			// this as an auth file.
			fns, err := filepath.Glob("search-console-*.json")
			if err != nil {
				log.Print("globbing failure when looking for cred file in $PWD", err)
			} else if len(fns) > 0 {
				scs, err = searchconsole.NewService(ctx, option.WithCredentialsFile(fns[0]))
			} else {
				fns, err := filepath.Glob(os.Getenv("HOME")+"/.search-console.json")
				if err != nil {
					log.Print("globbing failure when looking for cred file in $HOME", err)
				} else if len(fns) > 0 {
					scs, err = searchconsole.NewService(ctx, option.WithCredentialsFile(fns[0]))
				} else {
					log.Fatal("GOOGLE_APPLICATION_CREDENTIALS unset; no cred files found")
				}
			}
		}
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
		doQuerySinceAnalytics(false)
	case "query-full":
		doQuerySinceAnalytics(true)
	case "query-last":
		if len(os.Args) < n+2 {
			help(1)
		}
		if err := queryLastAnalytics(scs, mkSite(os.Args[n+1])); err != nil {
			log.Fatal(err)
		}
	case "query-day":
		if len(os.Args) < n+2 {
			help(1)
		}
		d := time.Now().UTC().Format(YYYYMMDD)
		if len(os.Args) == n+3 {
			d = os.Args[n+2]
		}
		d, err := parseDate(d)
		if err != nil {
			log.Fatal(err)
		}
		if d[0] == '-' {
			n, err := strconv.Atoi(d)
			if err != nil {
				log.Fatal("Invalid date shortcut: ", d)
			}
			d = time.Now().UTC().AddDate(0, 0, n).Format(YYYYMMDD)
		}
		if err := queryDayAnalytics(scs, mkSite(os.Args[n+1]), d); err != nil {
			log.Fatal(err)
		}
	case "query-keywords-full":
		// <site> <page>
		if len(os.Args) < n+3 {
			help(1)
		}
		if err := queryKeywordsFull(scs, mkSite(os.Args[n+1]), os.Args[n+2]); err != nil {
			log.Fatal(err)
		}
	case "help":
		help(0)
	default:
		help(1)
	}
}
