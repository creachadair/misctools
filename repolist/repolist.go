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
	host = flag.String("host", "github", "Repository host")

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
	hi, ok := hostMap[*host]
	if !ok {
		log.Fatalf("No query information known for host %q", *host)
	}
	var all stringset.Set
	for _, user := range flag.Args() {
		repos, err := hi.fetch(user)
		if err != nil {
			log.Fatalf("Fetching repository list for %q failed: %v", user, err)
		}
		all.Add(repos...)
	}
	fmt.Println(strings.Join(all.Elements(), "\n"))
}
