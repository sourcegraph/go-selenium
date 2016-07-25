package selenium

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"
)

var grid = flag.Bool("test.grid", false, "skip tests that fail on Selenium Grid")
var executor = flag.String("test.executor", defaultExecutor, "executor URL")
var browserName = flag.String("test.browserName", "firefox", "browser to run tests on")

func init() {
	flag.BoolVar(&Trace, "trace", false, "trace HTTP requests and responses")
	flag.Parse()

	caps["browserName"] = *browserName
}

var caps Capabilities = make(Capabilities)

var runOnSauce *bool = flag.Bool("saucelabs", false, "run on sauce")

func newRemote(testName string, t *testing.T) (wd WebDriver) {
	var err error
	if wd, err = NewRemote(caps, *executor); err != nil {
		t.Fatalf("can't start session for test %s: %s", testName, err)
	}
	return wd
}

func TestStatus(t *testing.T) {
	if *grid {
		t.Skip()
	}
	t.Parallel()
	wd := newRemote("TestStatus", t)
	defer wd.Quit()

	status, err := wd.Status()
	if err != nil {
		t.Fatal(err)
	}

	if status.OS.Name == "" {
		t.Fatal("No OS")
	}
}

func TestSessions(t *testing.T) {
	if *grid {
		t.Skip()
	}
	t.Parallel()
	wd := newRemote("TestSessions", t)
	defer wd.Quit()

	_, err := wd.Sessions()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewSession(t *testing.T) {
	t.Parallel()
	if *runOnSauce {
		return
	}
	wd := &remoteWebDriver{capabilities: caps, executor: *executor}
	sid, err := wd.NewSession()
	defer wd.Quit()

	if err != nil {
		t.Fatalf("error in new session - %s", err)
	}

	if sid == "" {
		t.Fatal("Empty session id")
	}

	if wd.id != sid {
		t.Fatal("Session id mismatch")
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestCapabilities", t)
	defer wd.Quit()

	c, err := wd.Capabilities()
	if err != nil {
		t.Fatal(err)
	}

	if c["browserName"] != caps["browserName"] {
		t.Fatalf("bad browser name - %s", c["browserName"])
	}
}

func TestSetTimeout(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestSetTimeout", t).T(t)
	defer wd.Quit()

	wd.SetTimeout("script", 200)
	wd.SetTimeout("implicit", 200)
	wd.SetTimeout("page load", 200)
}

func TestSetAsyncScriptTimeout(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestSetAsyncScriptTimeout", t).T(t)
	defer wd.Quit()

	wd.SetAsyncScriptTimeout(200)
}

func TestSetImplicitWaitTimeout(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestSetImplicitWaitTimeout", t).T(t)
	defer wd.Quit()

	wd.SetImplicitWaitTimeout(200)
}

func TestCurrentWindowHandle(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestCurrentWindowHandle", t).T(t)
	defer wd.Quit()

	handle := wd.CurrentWindowHandle()

	if handle == "" {
		t.Fatal("Empty handle")
	}
}

func TestWindowHandles(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestWindowHandles", t).T(t)
	defer wd.Quit()

	handles := wd.CurrentWindowHandle()

	if handles == "" {
		t.Fatal("No handles")
	}
}

func TestWindowSize(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestWindowSize", t).T(t)
	defer wd.Quit()

	size := wd.WindowSize(wd.CurrentWindowHandle())
	if size == nil || size.Height == 0 || size.Width == 0 {
		t.Fatal("Window size failed with size: %+v", size)
	}
}

func TestWindowPosition(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestWindowPosition", t).T(t)
	defer wd.Quit()

	pos := wd.WindowPosition(wd.CurrentWindowHandle())
	if pos == nil {
		t.Fatal("Window position failed")
	}
}

func TestResizeWindow(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestResizeWindow", t).T(t)
	defer wd.Quit()

	wd.ResizeWindow(wd.CurrentWindowHandle(), Size{400, 400})

	sz := wd.WindowSize(wd.CurrentWindowHandle())
	if int(sz.Width) != 400 {
		t.Fatalf("got width %f, want 400", sz.Width)
	}
	if int(sz.Height) != 400 {
		t.Fatalf("got height %f, want 400", sz.Height)
	}
}

func TestGet(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestGet", t).T(t)
	defer wd.Quit()

	wd.Get(serverURL)

	newURL := wd.CurrentURL()

	if newURL != serverURL {
		t.Fatalf("%s != %s", newURL, serverURL)
	}
}

func TestNavigation(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestNavigation", t).T(t)
	defer wd.Quit()

	url1 := serverURL
	wd.Get(url1)

	url2 := serverURL + "other"
	wd.Get(url2)

	wd.Back()
	url := wd.CurrentURL()
	if url != url1 {
		t.Fatalf("back got me to %s (expected %s)", url, url1)
	}
	wd.Forward()
	url = wd.CurrentURL()
	if url != url2 {
		t.Fatalf("forward got me to %s (expected %s)", url, url2)
	}

	wd.Refresh()
	url = wd.CurrentURL()
	if url != url2 {
		t.Fatalf("refresh got me to %s (expected %s)", url, url2)
	}
}

func TestTitle(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestTitle", t).T(t)
	defer wd.Quit()

	wd.Get(serverURL)
	title := wd.Title()
	expectedTitle := "Go Selenium Test Suite"
	if title != expectedTitle {
		t.Fatal("Bad title %s, should be %s", title, expectedTitle)
	}
}

func TestPageSource(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestPageSource", t).T(t)
	defer wd.Quit()

	wd.Get(serverURL)
	source := wd.PageSource()
	if !strings.Contains(source, "The home page.") {
		t.Fatalf("Bad source\n%s", source)
	}
}

type elementFinder interface {
	FindElement(by, value string) WebElementT
	FindElements(by, value string) []WebElementT
}

func TestFindElement(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestFindElement", t).T(t)
	defer wd.Quit()
	wd.Get(serverURL)
	testFindElement(t, wd, ByCSSSelector, "ol.list li", "foo")
}

func TestFindChildElement(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestFindChildElement", t).T(t)
	defer wd.Quit()
	wd.Get(serverURL)
	testFindElement(t, wd.FindElement(ByTagName, "body"), ByCSSSelector, "ol.list li", "foo")
}

func testFindElement(t *testing.T, ef elementFinder, by, value string, txt string) {
	elem := ef.FindElement(by, value)
	if want, got := txt, elem.Text(); want != got {
		t.Errorf("Elem for %q %q: want text %q, got %q", by, value, want, got)
	}
}

func TestFindElements(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestFindElements", t).T(t)
	defer wd.Quit()
	wd.Get(serverURL)
	testFindElements(t, wd, ByCSSSelector, "ol.list li", []string{"foo", "bar"})
}

func TestFindChildElements(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestFindChildElements", t).T(t)
	defer wd.Quit()
	wd.Get(serverURL)
	testFindElements(t, wd.FindElement(ByCSSSelector, "ol.list"), ByCSSSelector, "li", []string{"foo", "bar"})
}

func testFindElements(t *testing.T, ef elementFinder, by, value string, elemsTxt []string) {
	elems := ef.FindElements(by, value)
	if len(elems) != len(elemsTxt) {
		t.Fatal("Wrong number of elements %d (should be %d)", len(elems), len(elemsTxt))
	}
	t.Logf("Found %d elements for %q %q", len(elems), by, value)
	for i, txt := range elemsTxt {
		elem := elems[i]
		if want, got := txt, elem.Text(); want != got {
			t.Errorf("Elem %d for %q %q: want text %q, got %q", i, by, value, want, got)
		}
	}
}

func TestSendKeys(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestSendKeys", t).T(t)
	defer wd.Quit()

	wd.Get(serverURL)
	input := wd.FindElement(ByName, "q")
	input.SendKeys("golang\n")

	source := wd.PageSource()
	if !strings.Contains(source, "The Go Programming Language") {
		t.Fatal("Can't find Go")
	}
	if !strings.Contains(source, "golang") {
		t.Fatal("Can't find search query in source")
	}
}

func TestClick(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestClick", t).T(t)
	defer wd.Quit()

	wd.Get(serverURL)
	input := wd.FindElement(ByName, "q")
	input.SendKeys("golang")

	button := wd.FindElement(ById, "submit")
	button.Click()

	if !strings.Contains(wd.PageSource(), "The Go Programming Language") {
		t.Fatal("Can't find Go")
	}
}

func TestClick_Hidden(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestClick_Hidden", t)
	defer wd.Quit()

	if err := wd.Get(serverURL); err != nil {
		t.Fatal(err)
	}
	e, err := wd.FindElement(ByName, "hidden_name")
	if err != nil {
		t.Fatal(err)
	}
	err = e.Click()
	if err == nil {
		t.Fatal("expected clicking on hidden element to error")
	}
	want := "element not visible"
	if err.Error() != want {
		t.Fatalf("got error %v, want %v", err.Error(), want)
	}
}

func TestGetCookies(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestGetCookies", t).T(t)
	defer wd.Quit()

	wd.Get(serverURL)
	cookies := wd.GetCookies()

	if len(cookies) == 0 {
		t.Fatal("No cookies")
	}

	if cookies[0].Name == "" {
		t.Fatal("Empty cookie")
	}

	if cookies[0].Expiry != uint(cookieExpiry.Unix()) {
		t.Fatalf("Bad expiry time: expected %v, got %v", cookieExpiry, cookies[0].Expiry)
	}
}

func TestAddCookie(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestAddCookie", t).T(t)
	defer wd.Quit()

	wd.Get(serverURL)
	cookie := &Cookie{Name: "the nameless cookie", Value: "I have nothing"}
	wd.AddCookie(cookie)

	cookies := wd.GetCookies()
	for _, c := range cookies {
		if (c.Name == cookie.Name) && (c.Value == cookie.Value) {
			return
		}
	}

	t.Fatal("Can't find new cookie")
}

func TestDeleteAllCookies(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestDeleteCookie", t).T(t)
	defer wd.Quit()

	wd.Get(serverURL)
	cookies := wd.GetCookies()
	if len(cookies) == 0 {
		t.Fatal("No cookies")
	}

	wd.DeleteAllCookies()

	newCookies := wd.GetCookies()
	if len(newCookies) != 0 {
		t.Fatal("Cookies not deleted")
	}
}

func TestDeleteCookie(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestDeleteCookie", t).T(t)
	defer wd.Quit()

	wd.Get(serverURL)
	cookies := wd.GetCookies()
	if len(cookies) == 0 {
		t.Fatal("No cookies")
	}
	wd.DeleteCookie(cookies[0].Name)
	newCookies := wd.GetCookies()
	if len(newCookies) != len(cookies)-1 {
		t.Fatal("Cookie not deleted")
	}

	for _, c := range newCookies {
		if c.Name == cookies[0].Name {
			t.Fatal("Deleted cookie found")
		}
	}
}

func TestLocation(t *testing.T) {
	wd := newRemote("TestLocation", t).T(t)
	defer wd.Quit()

	wd.Get(serverURL)
	button := wd.FindElement(ById, "submit")

	loc := button.Location()

	if (loc.X == 0) || (loc.Y == 0) {
		t.Fatalf("Bad location: %v\n", loc)
	}
}

func TestLocationInView(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestLocationInView", t).T(t)
	defer wd.Quit()

	wd.Get(serverURL)
	button := wd.FindElement(ById, "submit")

	loc := button.LocationInView()

	if (loc.X == 0) || (loc.Y == 0) {
		t.Fatalf("Bad location: %v\n", loc)
	}
}

func TestSize(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestSize", t).T(t)
	defer wd.Quit()

	wd.Get(serverURL)
	button := wd.FindElement(ById, "submit")

	size := button.Size()

	if (size.Width == 0) || (size.Height == 0) {
		t.Fatalf("Bad size: %v\n", size)
	}
}

func TestExecuteScript(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestExecuteScript", t).T(t)
	defer wd.Quit()

	script := "return arguments[0] + arguments[1]"
	args := []interface{}{1, 2}
	reply := wd.ExecuteScript(script, args)

	result, ok := reply.(float64)
	if !ok {
		t.Fatal("Not an int reply")
	}

	if result != 3 {
		t.Fatal("Bad result %d (expected 3)", result)
	}
}

func TestScreenshot(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestScreenshot", t).T(t)
	defer wd.Quit()

	wd.Get(serverURL)
	dataReader := wd.Screenshot()

	data, err := ioutil.ReadAll(dataReader)
	if err != nil {
		t.Fatal("failed to read screenshot data")
	}

	if len(data) == 0 {
		t.Fatal("Empty reply")
	}
}

func TestIsSelected(t *testing.T) {
	t.Parallel()
	wd := newRemote("TestIsSelected", t).T(t)
	defer wd.Quit()

	wd.Get(serverURL)
	elem := wd.FindElement(ById, "chuk")

	selected := elem.IsSelected()
	if selected {
		t.Fatal("Already selected")
	}

	elem.Click()
	selected = elem.IsSelected()
	if !selected {
		t.Fatal("Not selected")
	}
}

// Test server

var homePage = `
<html>
<head>
	<title>Go Selenium Test Suite</title>
</head>
<body>
	The home page. <br />
	<form action="/search">
		<input name="q" /> <input type="submit" id="submit"/> <br />
		<input id="chuk" type="checkbox" /> A checkbox.
		<input type="hidden" name="hidden_name" />
	</form>
    <ol class="list">
      <li>foo</li>
      <li>bar</li>
    </ol>
    <ol class="otherlist">
      <li>baz</li>
      <li>qux</li>
    </ol>
</body>
</html>
`

var otherPage = `
<html>
<head>
	<title>Go Selenium Test Suite - Other Page</title>
</head>
<body>
	The other page.
</body>
</html>
`

var searchPage = `
<html>
<head>
	<title>Go Selenium Test Suite - Search Page</title>
</head>
<body>
	You searched for "%s". I'll pretend I've found:
	<p>
	"The Go Programming Language"
	</p>
</body>
</html>
`

var pages = map[string]string{
	"/":       homePage,
	"/other":  otherPage,
	"/search": searchPage,
}

var cookieExpiry = time.Now().Add(1 * time.Hour).UTC()

func handler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	page, ok := pages[path]
	if !ok {
		http.NotFound(w, r)
		return
	}

	if path == "/search" {
		r.ParseForm()
		page = fmt.Sprintf(page, r.Form["q"][0])
	}
	// Some cookies for the tests
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("cookie-%d", i)
		value := fmt.Sprintf("value-%d", i)
		http.SetCookie(w, &http.Cookie{Name: name, Value: value, Expires: cookieExpiry})
	}

	fmt.Fprintf(w, page)
}

var serverPort = ":4793"
var serverURL = "http://localhost" + serverPort + "/"

func init() {
	go func() {
		http.HandleFunc("/", handler)
		http.ListenAndServe(serverPort, nil)
	}()
}
