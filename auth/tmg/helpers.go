package tmg

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
)

var (
	storage = cache.New(5*time.Minute, 10*time.Minute)
)

// GetAuth : get auth
func GetAuth(c *AuthCnfg) (string, error) {
	if c.client == nil {
		c.client = &http.Client{}
	}

	parsedURL, err := url.Parse(c.SiteURL)
	if err != nil {
		return "", err
	}

	cacheKey := parsedURL.Host + "@tmg@" + c.Username + "@" + c.Password
	if accessToken, found := storage.Get(cacheKey); found {
		return accessToken.(string), nil
	}

	redirect, err := detectCookieAuthURL(c, c.SiteURL)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("%s://%s/CookieAuth.dll?Logon", parsedURL.Scheme, parsedURL.Host)

	params := url.Values{}

	querystr := strings.Replace(redirect.RawQuery, "GetLogon?", "", 1)
	for _, part := range strings.Split(querystr, "&") {
		p := strings.Split(part, "=")
		if len(p) == 2 {
			params.Set(p[0], p[1])
		}
	}

	params.Set("username", c.Username)
	params.Set("password", c.Password)

	// TODO: keepalive agent for https

	// client := &http.Client{
	// 	CheckRedirect: func(req *http.Request, via []*http.Request) error {
	// 		return http.ErrUseLastResponse
	// 	},
	// }
	c.client.CheckRedirect = doNotCheckRedirect

	resp, err := c.client.Post(endpoint, "application/x-www-form-urlencoded", strings.NewReader(params.Encode()))
	if err != nil {
		return "", err
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	io.Copy(ioutil.Discard, resp.Body)

	// fmt.Println(resp.StatusCode)
	authCookie := resp.Header.Get("Set-Cookie") // TODO: parse TMG cookie only (?)

	// TODO: ttl detection
	expiry := time.Hour
	storage.Set(cacheKey, authCookie, expiry)

	return authCookie, nil
}

func detectCookieAuthURL(c *AuthCnfg, siteURL string) (*url.URL, error) {
	if c.client == nil {
		c.client = &http.Client{}
	}

	// client := &http.Client{
	// 	CheckRedirect: func(req *http.Request, via []*http.Request) error {
	// 		return http.ErrUseLastResponse
	// 	},
	// }
	c.client.CheckRedirect = doNotCheckRedirect

	req, err := http.NewRequest("GET", siteURL, nil)
	if err != nil {
		return nil, err
	}

	// req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8")
	// req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/71.0.3578.98 Safari/537.36")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	io.Copy(ioutil.Discard, resp.Body)

	redirect, err := resp.Location()
	if err != nil {
		return nil, err
	}

	return redirect, nil
}

// doNotCheckRedirect *http.Client CheckRedirect callback to ignore redirects
func doNotCheckRedirect(req *http.Request, via []*http.Request) error {
	return http.ErrUseLastResponse
}
