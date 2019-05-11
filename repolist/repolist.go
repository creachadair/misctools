// Program repolist prints a list of the public repositories owned by one or
// more users or organizations on common hosting sites.  It currently
// understands the GitHub and Bitbucket APIs.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"bitbucket.org/creachadair/stringset"
	"bitbucket.org/creachadair/vql"
)

var (
	repoHost     = flag.String("host", "github", "Default repository host")
	includeForks = flag.Bool("forks", false, "Include forks in listing")
	doJSON       = flag.Bool("json", false, "Emit output as JSON objects")
	authToken    = flag.String("auth", "", "Use this username:token to authenticate to the host")

	hostMap = map[string]hostInfo{
		"github": {
			url: "https://api.github.com/users/{}/repos?per_page=100",
			query: vql.Each(vql.Bind(map[string]vql.Query{
				"url":    vql.Key("html_url"),
				"desc":   vql.Or{vql.Key("description"), vql.Const("")},
				"isFork": vql.Key("fork"),
			})),
		},
		"bitbucket": {
			url: "https://api.bitbucket.org/2.0/repositories/{}?pagelen=100",
			query: vql.Seq{
				vql.Key("values"),
				vql.Each(vql.Bind(map[string]vql.Query{
					"url":    vql.Keys("links", "html", "href"),
					"desc":   vql.Or{vql.Key("description"), vql.Const("")},
					"isFork": vql.Seq{vql.Key("parent"), vql.As(vql.NotNil)},
				})),
			},
		},
	}
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %s [-host site] user ...

List the names of public Git repositories owned by the specified users on
well-known hosting sites. By default the -host flag determines which site
applies to each user; or use "user@site" to specify a different one per user.

By default, API requests are made without authentication. Set the -auth flag to
"username:token" to authenticate the request with those credentials.

With -json, each repository is written out as a JSON object containing keys:

  url:    The fetch URL for the repository (string)
  isFork: Whether the repository is a fork (bool)
  desc:   The description of the repository (string)

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

type hostInfo struct {
	url   string
	query vql.Query
}

func (h hostInfo) fetch(user string) ([]string, error) {
	query := strings.ReplaceAll(h.url, "{}", user)
	req, err := http.NewRequest("GET", query, nil)
	if err != nil {
		return nil, fmt.Errorf("http request: %v", err)
	}

	// Check for authorization credentials.
	if *authToken != "" {
		parts := strings.SplitN(*authToken, ":", 2)
		if len(parts) != 2 {
			return nil, errors.New("invalid auth token format")
		}
		req.SetBasicAuth(parts[0], parts[1])
	}

	// Issue the query and recover the JSON response blob.
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %v", err)
	}
	defer rsp.Body.Close()

	data, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return nil, fmt.Errorf("http body: %v", err)
	}
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http get: %s (%s)", rsp.Status, string(data))
	}

	// Decode the JSON response into structures and evaluate the host query to
	// extract repository names and details.
	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("json unmarshal: %v", err)
	}
	v, err := vql.Eval(h.query, obj)
	if err != nil {
		return nil, fmt.Errorf("eval: %v", err)
	}
	var names []string
	for _, elt := range v.([]interface{}) {
		repo := elt.(map[string]interface{})
		if repo["isFork"].(bool) && !*includeForks {
			continue
		}
		if *doJSON {
			bits, err := json.Marshal(repo)
			if err != nil {
				return nil, fmt.Errorf("render: %v", err)
			}
			names = append(names, string(bits))
		} else {
			names = append(names, repo["url"].(string))
		}
	}
	return names, nil
}

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		log.Fatalf("Usage: %s [-host site] user ...", filepath.Base(os.Args[0]))
	}

	var all stringset.Set
	for _, user := range flag.Args() {
		site := *repoHost
		if uhost := strings.SplitN(user, "@", 2); len(uhost) == 2 {
			user, site = uhost[0], uhost[1]
		}
		hi, ok := hostMap[site]
		if !ok {
			log.Fatalf("No query information for host site %q", site)
		}

		repos, err := hi.fetch(user)
		if err != nil {
			log.Fatalf("Fetching %s repository list for %q failed: %v", site, user, err)
		}
		all.Add(repos...)
	}
	fmt.Println(strings.Join(all.Elements(), "\n"))
}
