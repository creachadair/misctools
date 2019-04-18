// Program repolist prints a list of the public repositories owned by one or
// more users or organizations on common hosting sites.
//
// This program requires jq to be installed (https://stedolan.github.io/jq/).
// It currently understands the GitHub and Bitbucket APIs.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bitbucket.org/creachadair/stringset"
)

var (
	host = flag.String("host", "github", "Default repository host")

	hostMap = map[string]hostInfo{
		"github": {
			url:   "https://api.github.com/users/{}/repos",
			query: ".[]|select(.fork|not)|.html_url",
		},
		"bitbucket": {
			url:   "https://api.bitbucket.org/2.0/repositories/{}",
			query: ".values[].links.html.href",
		},
	}
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %s [-host site] user ...

Fetch the names of public Git repositories owned by the specified users on
well-known hosting sites. By default the -host flag determines which site
applies to each user; or use "user@site" to specify a different one per user.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

type hostInfo struct {
	url   string
	query string
}

func (h hostInfo) fetch(user string) ([]string, error) {
	query := strings.ReplaceAll(h.url, "{}", user)
	rsp, err := http.Get(query)
	if err != nil {
		return nil, fmt.Errorf("http get: %v", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http get: %s", rsp.Status)
	}
	cmd := exec.Command("jq", "-r", h.query)
	cmd.Stdin = rsp.Body
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("jq parse: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n"), nil
}

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		log.Fatalf("Usage: %s [-host site] user ...", filepath.Base(os.Args[0]))
	}

	var all stringset.Set
	for _, user := range flag.Args() {
		site := *host
		if uhost := strings.SplitN(user, "@", 2); len(uhost) == 2 {
			user, site = uhost[0], uhost[1]
		}
		hi, ok := hostMap[site]
		if !ok {
			log.Fatalf("No query information for host site %q", site)
		}

		log.Printf("Fetching %s repositories for %q...", site, user)
		repos, err := hi.fetch(user)
		if err != nil {
			log.Fatalf("Fetching %s repository list for %q failed: %v", site, user, err)
		}
		all.Add(repos...)
	}
	fmt.Println(strings.Join(all.Elements(), "\n"))
}
