/* Remote Selenium client implementation.

See http://code.google.com/p/selenium/wiki/JsonWireProtocol for wire protocol.
*/

package selenium

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
)

var Log = log.New(os.Stderr, "[selenium] ", log.Ltime|log.Lmicroseconds)
var Trace bool

/* Errors returned by Selenium server. */
var errorCodes = map[int]string{
	7:  "no such element",
	8:  "no such frame",
	9:  "unknown command",
	10: "stale element reference",
	11: "element not visible",
	12: "invalid element state",
	13: "unknown error",
	15: "element is not selectable",
	17: "javascript error",
	19: "xpath lookup error",
	21: "timeout",
	23: "no such window",
	24: "invalid cookie domain",
	25: "unable to set cookie",
	26: "unexpected alert open",
	27: "no alert open",
	28: "script timeout",
	29: "invalid element coordinates",
	32: "invalid selector",
}

const (
	SUCCESS          = 0
	DEFAULT_EXECUTOR = "http://127.0.0.1:4444/wd/hub"
	jsonMIMEType     = "application/json"
)

type remoteWD struct {
	id, executor string
	capabilities Capabilities
	// FIXME
	// profile             BrowserProfile
}

/* Server reply */
type serverReply struct {
	SessionId string
	Status    int
	Value     json.RawMessage
}

type reply serverReply // TODO(sqs): redundant

func (r *reply) readValue(v interface{}) error {
	return json.Unmarshal(r.Value, v)
}

/* Various reply types, we use them to json.Unmarshal replies */
type stringReply struct {
	Value *string
}
type stringsReply struct {
	Value []string
}
type boolReply struct {
	Value bool
}
type element struct {
	ELEMENT string
}
type elementReply struct {
	Value element
}
type elementsReply struct {
	Value []element
}
type cookiesReply struct {
	Value []Cookie
}
type locationReply struct {
	Value Point
}
type sizeReply struct {
	Value Size
}
type anyReply struct {
	Value interface{}
}
type capabilitiesReply struct {
	Value Capabilities
}

func (wd *remoteWD) url(template string, args ...interface{}) string {
	path := fmt.Sprintf(template, args...)
	return wd.executor + path
}

var httpClient = http.Client{
	// WebDriver requires that all requests have an 'Accept: application/json' header. We must add
	// it here because by default net/http will not include that header when following redirects.
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		req.Header.Add("Accept", jsonMIMEType)
		return nil
	},
}

func (wd *remoteWD) send(method, url string, data []byte) (r *reply, err error) {
	var buf []byte
	if buf, err = wd.execute(method, url, data); err == nil {
		if len(buf) > 0 {
			err = json.Unmarshal(buf, &r)
		}
	}
	return
}

func (wd *remoteWD) execute(method, url string, data []byte) ([]byte, error) {
	Log.Printf("-> %s %s [%d bytes]", method, url, len(data))
	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", jsonMIMEType)

	if Trace {
		if dump, err := httputil.DumpRequest(req, true); err == nil {
			Log.Printf("-> TRACE\n%s", dump)
		}
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if Trace {
		if dump, err := httputil.DumpResponse(res, true); err == nil {
			Log.Printf("<- TRACE\n%s", dump)
		}
	}

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	Log.Printf("<- %s (%s) [%d bytes]", res.Status, res.Header["Content-Type"], len(buf))

	if res.StatusCode >= 400 {
		reply := new(serverReply)
		err := json.Unmarshal(buf, reply)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Bad server reply status: %s", res.Status))
		}
		message, ok := errorCodes[reply.Status]
		if !ok {
			message = fmt.Sprintf("unknown error - %d", reply.Status)
		}

		return nil, errors.New(message)
	}

	/* Some bug(?) in Selenium gets us nil values in output, json.Unmarshal is
	* not happy about that.
	 */
	if strings.HasPrefix(res.Header.Get("Content-Type"), jsonMIMEType) {
		reply := new(serverReply)
		err := json.Unmarshal(buf, reply)
		if err != nil {
			return nil, err
		}

		if reply.Status != SUCCESS {
			message, ok := errorCodes[reply.Status]
			if !ok {
				message = fmt.Sprintf("unknown error - %d", reply.Status)
			}

			return nil, errors.New(message)
		}
		return buf, err
	}

	// Nothing was returned, this is OK for some commands
	return buf, nil
}

/* Create new remote client, this will also start a new session.
   capabilities - the desired capabilities, see http://goo.gl/SNlAk
   executor - the URL to the Selenim server
*/
func NewRemote(capabilities Capabilities, executor string) (WebDriver, error) {

	if executor == "" {
		executor = DEFAULT_EXECUTOR
	}

	wd := &remoteWD{executor: executor, capabilities: capabilities}
	// FIXME: Handle profile

	_, err := wd.NewSession()
	if err != nil {
		return nil, err
	}

	return wd, nil
}

func (wd *remoteWD) stringCommand(urlTemplate string) (string, error) {
	url := wd.url(urlTemplate, wd.id)
	res, err := wd.execute("GET", url, nil)
	if err != nil {
		return "", err
	}

	reply := new(stringReply)
	err = json.Unmarshal(res, reply)
	if err != nil {
		return "", err
	}

	return *reply.Value, nil
}

func (wd *remoteWD) voidCommand(urlTemplate string, data []byte) error {
	url := wd.url(urlTemplate, wd.id)
	_, err := wd.execute("POST", url, data)
	return err

}

func (wd remoteWD) stringsCommand(urlTemplate string) ([]string, error) {
	url := wd.url(urlTemplate, wd.id)
	res, err := wd.execute("GET", url, nil)
	if err != nil {
		return nil, err
	}
	reply := new(stringsReply)
	err = json.Unmarshal(res, reply)
	if err != nil {
		return nil, err
	}

	return reply.Value, nil
}

func (wd *remoteWD) boolCommand(urlTemplate string) (bool, error) {
	url := wd.url(urlTemplate, wd.id)
	res, err := wd.execute("GET", url, nil)
	if err != nil {
		return false, err
	}

	reply := new(boolReply)
	err = json.Unmarshal(res, reply)
	if err != nil {
		return false, err
	}

	return reply.Value, nil
}

// WebDriver interface implementation

func (wd *remoteWD) Status() (v *Status, err error) {
	var r *reply
	if r, err = wd.send("GET", wd.url("/status"), nil); err == nil {
		err = r.readValue(&v)
	}
	return
}

func (wd *remoteWD) NewSession() (sessionId string, err error) {
	message := map[string]interface{}{
		"desiredCapabilities": wd.capabilities,
	}
	var data []byte
	if data, err = json.Marshal(message); err != nil {
		return
	}
	if r, err := wd.send("POST", wd.url("/session"), data); err == nil {
		sessionId = r.SessionId
		wd.id = r.SessionId
	}
	return
}

func (wd *remoteWD) Capabilities() (Capabilities, error) {
	res, err := wd.execute("GET", wd.url("/session/%s", wd.id), nil)
	if err != nil {
		return nil, err
	}

	c := new(capabilitiesReply)
	err = json.Unmarshal(res, c)
	if err != nil {
		return nil, err
	}

	return c.Value, nil
}

func (wd *remoteWD) SetAsyncScriptTimeout(ms uint) error {
	params := map[string]uint{
		"ms": ms,
	}

	data, err := json.Marshal(params)
	if err != nil {
		return err
	}

	return wd.voidCommand("/session/%s/timeouts/async_script", data)
}

func (wd *remoteWD) SetImplicitWaitTimeout(ms uint) error {
	params := map[string]uint{
		"ms": ms,
	}

	data, err := json.Marshal(params)
	if err != nil {
		return err
	}

	return wd.voidCommand("/session/%s/timeouts/implicit_wait", data)
}

func (wd *remoteWD) AvailableEngines() ([]string, error) {
	return wd.stringsCommand("/session/%s/ime/available_engines")
}

func (wd *remoteWD) ActiveEngine() (string, error) {
	return wd.stringCommand("/session/%s/ime/active_engine")
}

func (wd *remoteWD) IsEngineActivated() (bool, error) {
	return wd.boolCommand("/session/%s/ime/activated")
}

func (wd *remoteWD) DeactivateEngine() error {
	return wd.voidCommand("session/%s/ime/deactivate", nil)
}

func (wd *remoteWD) ActivateEngine(engine string) error {
	params := map[string]string{
		"engine": engine,
	}

	data, err := json.Marshal(params)
	if err != nil {
		return err
	}

	return wd.voidCommand("/session/%s/ime/activate", data)
}

func (wd *remoteWD) Quit() error {
	_, err := wd.execute("DELETE", wd.url("/session/%s", wd.id), nil)
	if err == nil {
		wd.id = ""
	}

	return err
}

func (wd *remoteWD) CurrentWindowHandle() (string, error) {
	return wd.stringCommand("/session/%s/window_handle")
}

func (wd *remoteWD) WindowHandles() ([]string, error) {
	return wd.stringsCommand("/session/%s/window_handles")
}

func (wd *remoteWD) CurrentURL() (string, error) {
	res, err := wd.execute("GET", wd.url("/session/%s/url", wd.id), nil)
	if err != nil {
		return "", err
	}
	reply := new(stringReply)
	json.Unmarshal(res, reply)

	return *reply.Value, nil

}

func (wd *remoteWD) Get(url string) error {
	params := map[string]string{
		"url": url,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	_, err = wd.execute("POST", wd.url("/session/%s/url", wd.id), data)

	return err
}

func (wd *remoteWD) Forward() error {
	return wd.voidCommand("/session/%s/forward", nil)
}

func (wd *remoteWD) Back() error {
	return wd.voidCommand("/session/%s/back", nil)
}

func (wd *remoteWD) Refresh() error {
	return wd.voidCommand("/session/%s/refresh", nil)
}

func (wd *remoteWD) Title() (string, error) {
	return wd.stringCommand("/session/%s/title")
}

func (wd *remoteWD) PageSource() (string, error) {
	return wd.stringCommand("/session/%s/source")
}

func (wd *remoteWD) find(by, value, suffix, url string) ([]byte, error) {
	params := map[string]string{
		"using": by,
		"value": value,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	if url == "" {
		url = "/session/%s/element"
	}

	urlTemplate := url + suffix
	url = wd.url(urlTemplate, wd.id)
	return wd.execute("POST", url, data)
}

func decodeElement(wd *remoteWD, data []byte) (WebElement, error) {
	reply := new(elementReply)
	err := json.Unmarshal(data, reply)
	if err != nil {
		return nil, err
	}

	elem := &remoteWE{wd, reply.Value.ELEMENT}
	return elem, nil
}

func (wd *remoteWD) FindElement(by, value string) (WebElement, error) {
	res, err := wd.find(by, value, "", "")
	if err != nil {
		return nil, err
	}

	return decodeElement(wd, res)
}

func decodeElements(wd *remoteWD, data []byte) ([]WebElement, error) {
	reply := new(elementsReply)
	err := json.Unmarshal(data, reply)
	if err != nil {
		return nil, err
	}

	elems := make([]WebElement, len(reply.Value))
	for i, elem := range reply.Value {
		elems[i] = &remoteWE{wd, elem.ELEMENT}
	}

	return elems, nil
}

func (wd *remoteWD) FindElements(by, value string) ([]WebElement, error) {
	res, err := wd.find(by, value, "s", "")
	if err != nil {
		return nil, err
	}

	return decodeElements(wd, res)
}

func (wd *remoteWD) Close() error {
	_, err := wd.execute("DELETE", wd.url("/session/%s/window", wd.id), nil)
	return err
}

func (wd *remoteWD) SwitchWindow(name string) error {
	params := map[string]string{
		"name": name,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return wd.voidCommand("/session/%s/window", data)
}

func (wd *remoteWD) CloseWindow(name string) error {
	_, err := wd.execute("DELETE", wd.url("/session/%s/window", wd.id), nil)
	return err
}

func (wd *remoteWD) SwitchFrame(frame string) error {
	params := map[string]string{
		"id": frame,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return wd.voidCommand("/session/%s/frame", data)
}

func (wd *remoteWD) ActiveElement() (WebElement, error) {
	url := wd.url("/session/%s/element/active", wd.id)
	res, err := wd.execute("GET", url, nil)
	if err != nil {
		return nil, err
	}

	return decodeElement(wd, res)
}

func (wd *remoteWD) GetCookies() ([]Cookie, error) {
	data, err := wd.execute("GET", wd.url("/session/%s/cookie", wd.id), nil)
	if err != nil {
		return nil, err
	}

	reply := new(cookiesReply)
	err = json.Unmarshal(data, reply)
	if err != nil {
		return nil, err
	}

	return reply.Value, nil
}

func (wd *remoteWD) AddCookie(cookie *Cookie) error {
	params := map[string]*Cookie{
		"cookie": cookie,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}

	return wd.voidCommand("/session/%s/cookie", data)
}

func (wd *remoteWD) DeleteAllCookies() error {
	_, err := wd.execute("DELETE", wd.url("/session/%s/cookie", wd.id), nil)
	return err
}

func (wd *remoteWD) DeleteCookie(name string) error {
	_, err := wd.execute("DELETE", wd.url("/session/%s/cookie/%s", wd.id, name), nil)
	return err
}

func (wd *remoteWD) Click(button int) error {
	params := map[string]int{
		"button": button,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return wd.voidCommand("/session/%s/click", data)
}

func (wd *remoteWD) DoubleClick() error {
	return wd.voidCommand("/session/%s/doubleclick", nil)
}

func (wd *remoteWD) ButtonDown() error {
	return wd.voidCommand("/session/%s/buttondown", nil)
}

func (wd *remoteWD) ButtonUp() error {
	return wd.voidCommand("/session/%s/buttonup", nil)
}

func (wd *remoteWD) SendModifier(modifier string, isDown bool) error {
	params := map[string]interface{}{
		"value":  modifier,
		"isdown": isDown,
	}

	data, err := json.Marshal(params)
	if err != nil {
		return err
	}

	return wd.voidCommand("/session/%s/modifier", data)
}

func (wd *remoteWD) DismissAlert() error {
	return wd.voidCommand("/session/%s/dismiss_alert", nil)
}

func (wd *remoteWD) AcceptAlert() error {
	return wd.voidCommand("/session/%s/accept_alert", nil)
}

func (wd *remoteWD) AlertText() (string, error) {
	return wd.stringCommand("/session/%s/alert_text")
}

func (wd *remoteWD) SetAlertText(text string) error {
	params := map[string]string{
		"text": text,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}

	return wd.voidCommand("/session/%s/alert_text", data)
}

func (wd *remoteWD) execScript(script string, args []interface{}, suffix string) (interface{}, error) {
	params := map[string]interface{}{
		"script": script,
		"args":   args,
	}

	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	template := "/session/%s/execute" + suffix
	url := wd.url(template, wd.id)
	res, err := wd.execute("POST", url, data)
	if err != nil {
		return nil, err
	}

	reply := new(anyReply)
	err = json.Unmarshal(res, reply)
	if err != nil {
		return nil, err
	}

	return reply.Value, nil
}

func (wd *remoteWD) ExecuteScript(script string, args []interface{}) (interface{}, error) {
	return wd.execScript(script, args, "")
}

func (wd *remoteWD) ExecuteScriptAsync(script string, args []interface{}) (interface{}, error) {
	return wd.execScript(script, args, "_async")
}

func (wd *remoteWD) Screenshot() ([]byte, error) {
	data, err := wd.stringCommand("/session/%s/screenshot")
	if err != nil {
		return nil, err
	}

	// Selenium returns base64 encoded image
	buf := []byte(data)
	decoder := base64.NewDecoder(base64.StdEncoding, bytes.NewBuffer(buf))
	return ioutil.ReadAll(decoder)
}

// WebElement interface implementation

type remoteWE struct {
	parent *remoteWD
	id     string
}

func (elem *remoteWE) Click() error {
	urlTemplate := fmt.Sprintf("/session/%%s/element/%s/click", elem.id)
	return elem.parent.voidCommand(urlTemplate, nil)
}

func (elem *remoteWE) SendKeys(keys string) error {
	chars := make([]string, len(keys))
	for i, c := range keys {
		chars[i] = string(c)
	}
	params := map[string][]string{
		"value": chars,
	}

	data, err := json.Marshal(params)
	if err != nil {
		return err
	}

	urlTemplate := fmt.Sprintf("/session/%%s/element/%s/value", elem.id)
	return elem.parent.voidCommand(urlTemplate, data)
}

func (elem *remoteWE) TagName() (string, error) {
	urlTemplate := fmt.Sprintf("/session/%%s/element/%s/name", elem.id)
	return elem.parent.stringCommand(urlTemplate)
}

func (elem *remoteWE) Text() (string, error) {
	urlTemplate := fmt.Sprintf("/session/%%s/element/%s/text", elem.id)
	return elem.parent.stringCommand(urlTemplate)
}

func (elem *remoteWE) Submit() error {
	urlTemplate := fmt.Sprintf("/session/%%s/element/%s/submit", elem.id)
	return elem.parent.voidCommand(urlTemplate, nil)
}

func (elem *remoteWE) Clear() error {
	urlTemplate := fmt.Sprintf("/session/%%s/element/%s/clear", elem.id)
	return elem.parent.voidCommand(urlTemplate, nil)
}

func (elem *remoteWE) MoveTo(xOffset, yOffset int) error {
	params := map[string]interface{}{
		"element": elem.id,
		"xoffset": xOffset,
		"yoffset": yOffset,
	}
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return elem.parent.voidCommand("/session/%s/moveto", data)
}

func (elem *remoteWE) FindElement(by, value string) (WebElement, error) {
	res, err := elem.parent.find(by, value, "", fmt.Sprintf("/session/%%s/element/%s/element", elem.id))
	if err != nil {
		return nil, err
	}

	return decodeElement(elem.parent, res)
}

func (elem *remoteWE) FindElements(by, value string) ([]WebElement, error) {
	res, err := elem.parent.find(by, value, "s", fmt.Sprintf("/session/%%s/element/%s/element", elem.id))
	if err != nil {
		return nil, err
	}

	return decodeElements(elem.parent, res)
}

func (elem *remoteWE) boolQuery(urlTemplate string) (bool, error) {
	url := fmt.Sprintf(urlTemplate, elem.id)
	return elem.parent.boolCommand(url)
}

// Porperties
func (elem *remoteWE) IsSelected() (bool, error) {
	return elem.boolQuery("/session/%%s/element/%s/selected")
}

func (elem *remoteWE) IsEnabled() (bool, error) {
	return elem.boolQuery("/session/%%s/element/%s/enabled")
}

func (elem *remoteWE) IsDiaplayed() (bool, error) {
	return elem.boolQuery("/session/%%s/element/%s/displayed")
}

func (elem *remoteWE) GetAttribute(name string) (string, error) {
	template := "/session/%%s/element/%s/attribute/%s"
	urlTemplate := fmt.Sprintf(template, elem.id, name)

	return elem.parent.stringCommand(urlTemplate)
}

func (elem *remoteWE) location(suffix string) (*Point, error) {
	wd := elem.parent
	path := "/session/%s/element/%s/location" + suffix
	url := wd.url(path, wd.id, elem.id)
	res, err := wd.execute("GET", url, nil)
	if err != nil {
		return nil, err
	}
	reply := new(locationReply)
	err = json.Unmarshal(res, reply)
	if err != nil {
		return nil, err
	}

	return &reply.Value, nil
}

func (elem *remoteWE) Location() (*Point, error) {
	return elem.location("")
}

func (elem *remoteWE) LocationInView() (*Point, error) {
	return elem.location("_in_view")
}

func (elem *remoteWE) Size() (*Size, error) {
	wd := elem.parent
	res, err := wd.execute("GET", wd.url("/session/%s/element/%s/size", wd.id, elem.id), nil)
	if err != nil {
		return nil, err
	}
	reply := new(sizeReply)
	err = json.Unmarshal(res, reply)
	if err != nil {
		return nil, err
	}

	return &reply.Value, nil
}

func (elem *remoteWE) CSSProperty(name string) (string, error) {
	wd := elem.parent
	urlTemplate := fmt.Sprintf("/session/%s/element/%s/css/%s", wd.id, elem.id, name)
	return elem.parent.stringCommand(urlTemplate)
}
