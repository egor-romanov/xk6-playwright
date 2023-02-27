package playwright

import (
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/tidwall/gjson"
	"go.k6.io/k6/js/modules"
)

// Register the extension on module initialization, available to
// import from JS as "k6/x/playwright".
func init() {
	modules.Register("k6/x/playwright", new(Playwright))
}

// Playwright is the k6 extension for a playwright-go client.
type Playwright struct {
	Self           *playwright.Playwright
	Browser        playwright.Browser
	BrowserContext playwright.BrowserContext
	Page           playwright.Page
}

// Launch starts the playwright client and launches a browser
func (p *Playwright) Launch(args playwright.BrowserTypeLaunchOptions) error {
	pw, err := playwright.Run()
	if err != nil {
		ReportError(err, "xk6-playwright: cannot start playwright")
		return err
	}
	browser, err := pw.Firefox.Launch(args)
	if err != nil {
		ReportError(err, "xk6-playwright: cannot launch chromium")
		return err
	}
	p.Self = pw
	p.Browser = browser
	return nil
}

// LaunchPersistent starts the playwright client and launches a browser with a persistent context
func (p *Playwright) LaunchPersistent(dir string, args playwright.BrowserTypeLaunchPersistentContextOptions) error {
	pw, err := playwright.Run()
	if err != nil {
		ReportError(err, "xk6-playwright: cannot start playwright")
		return err
	}
	browser, err := pw.Firefox.LaunchPersistentContext(dir, args)
	if err != nil {
		ReportError(err, "xk6-playwright: cannot launch chromium")
		return err
	}
	p.Self = pw
	p.BrowserContext = browser
	return nil
}

// Connect attaches Playwright to an existing browser instance
func (p *Playwright) Connect(url string, args playwright.BrowserTypeConnectOverCDPOptions) error {
	pw, err := playwright.Run()
	if err != nil {
		ReportError(err, "xk6-playwright: cannot start playwright")
		return err
	}
	browser, err := pw.Firefox.ConnectOverCDP(url, args)
	if err != nil {
		ReportError(err, "xk6-playwright: cannot launch chromium")
		return err
	}
	context := browser.Contexts()[0]

	p.Self = pw
	p.Browser = browser
	p.Page = context.Pages()[0]
	return nil
}

// NewPage opens a new page within the browser
func (p *Playwright) NewPage() error {
	page, err := p.newPage()
	if err != nil {
		ReportError(err, "xk6-playwright: cannot create page")
		return err
	}
	p.Page = page
	return nil
}

// Kill closes browser instance and stops puppeteer client
func (p *Playwright) Kill() error {
	if err := p.closeBrowser(); err != nil {
		ReportError(err, "xk6-playwright: cannot close browser")
		return err
	}
	if err := p.Self.Stop(); err != nil {
		ReportError(err, "xk6-playwright: cannot stop playwright")
		return err
	}
	return nil
}

//---------------------------------------------------------------------
//                         ACTIONS
//---------------------------------------------------------------------

// Goto wrapper around playwright goto page function that takes in a url and a set of options
func (p *Playwright) Goto(url string, opts playwright.PageGotoOptions) error {
	if _, err := p.Page.Goto(url, opts); err != nil {
		ReportError(err, "xk6-playwright: error when goto url")
		return err
	}
	return nil
}

// WaitForSelector wrapper around playwright waitForSelector page function that takes in a selector and a set of options
func (p *Playwright) WaitForSelector(selector string, opts playwright.PageWaitForSelectorOptions) error {
	if _, err := p.Page.WaitForSelector(selector, opts); err != nil {
		ReportError(err, "xk6-playwright: error waiting for selector")
		return err
	}
	return nil
}

func (p *Playwright) WaitForNavigation(opts playwright.PageWaitForNavigationOptions) error {
	if _, err := p.Page.WaitForNavigation(opts); err != nil {
		ReportError(err, "xk6-playwright: error waiting for navigation")
		return err
	}
	return nil
}

func (p *Playwright) WaitForLoadState(state string) {
	p.Page.WaitForLoadState(state)
}

func (p *Playwright) CountAll(selector string) (int32, error) {
	elements, err := p.Page.QuerySelectorAll(selector)
	if err != nil {
		ReportError(err, "xk6-playwright: error querying selector")
		return 0, err
	}
	return int32(len(elements)), nil
}

func (p *Playwright) CountByState(selector string, state string) (int32, error) {
	elements, err := p.Page.QuerySelectorAll(selector)
	if err != nil {
		ReportError(err, "xk6-playwright: error querying selector")
		return 0, err
	}
	var count int32
	for _, element := range elements {
		shouldCount := false
		var err error
		switch state {
		case "visible":
			shouldCount, err = element.IsVisible()
		case "hidden":
			shouldCount, err = element.IsHidden()
		case "enabled":
			shouldCount, err = element.IsEnabled()
		case "disabled":
			shouldCount, err = element.IsDisabled()
		case "editable":
			shouldCount, err = element.IsEditable()
		case "checked":
			shouldCount, err = element.IsChecked()
		default:
			err = errors.New("invalid state")
			ReportError(err, "xk6-playwright: invalid state")
			return 0, err
		}

		if err != nil {
			ReportError(err, "xk6-playwright: error checking visibility")
			return 0, err
		}
		if shouldCount {
			count++
		}
	}
	return count, nil
}

// Click wrapper around playwright click page function that takes in a selector and a set of options
func (p *Playwright) Click(selector string, opts playwright.PageClickOptions) error {
	if err := p.Page.Click(selector, opts); err != nil {
		ReportError(err, "xk6-playwright: error with clicking")
		return err
	}
	return nil
}

// Type wrapper around playwright type page function that takes in a selector, string, and a set of options
func (p *Playwright) Type(selector string, typedString string, opts playwright.PageTypeOptions) error {
	if err := p.Page.Type(selector, typedString, opts); err != nil {
		ReportError(err, "xk6-playwright: error with typing")
		return err
	}
	return nil
}

// PressKey wrapper around playwright Press page function that takes in a selector, key, and a set of options
func (p *Playwright) PressKey(selector string, key string, opts playwright.PagePressOptions) error {
	if err := p.Page.Press(selector, key, opts); err != nil {
		ReportError(err, "xk6-playwright: error with pressing the key")
		return err
	}
	return nil
}

// Sleep wrapper around playwright waitForTimeout page function that sleeps for the given `timeout` in milliseconds
func (p *Playwright) Sleep(time float64) {
	p.Page.WaitForTimeout(time)
}

// Screenshot wrapper around playwright screenshot page function that attempts to take and save a png image of the current screen.
func (p *Playwright) Screenshot(filename string, perm fs.FileMode, opts playwright.PageScreenshotOptions) error {
	image, err := p.Page.Screenshot(opts)
	if err != nil {
		ReportError(err, "xk6-playwright: error with taking a screenshot")
		return err
	}
	err = ioutil.WriteFile("Screenshot_"+time.Now().Format("2017-09-07 17:06:06")+".png", image, perm)
	if err != nil {
		ReportError(err, "xk6-playwright: error with writing the screenshot to the file system")
		return err
	}
	return nil
}

// Focus wrapper around playwright focus page function that takes in a selector and a set of options
func (p *Playwright) Focus(selector string, opts playwright.PageFocusOptions) error {
	if err := p.Page.Focus(selector); err != nil {
		ReportError(err, "xk6-playwright: error with focusing")
		return err
	}
	return nil
}

// Fill wrapper around playwright fill page function that takes in a selector, text, and a set of options
func (p *Playwright) Fill(selector string, filledString string, opts playwright.FrameFillOptions) error {
	if err := p.Page.Fill(selector, filledString, opts); err != nil {
		ReportError(err, "xk6-playwright: error with filling")
		return err
	}
	return nil
}

// SelectOptions wrapper around playwright selectOptions page function that takes in a selector, values, and a set of options
func (p *Playwright) SelectOptions(selector string, values playwright.SelectOptionValues, opts playwright.FrameSelectOptionOptions) error {
	_, err := p.Page.SelectOption(selector, values, opts)
	if err != nil {
		ReportError(err, "xk6-playwright: error with selecting options")
		return err
	}
	return nil
}

// Check wrapper around playwright check page function that takes in a selector and a set of options
func (p *Playwright) Check(selector string, opts playwright.FrameCheckOptions) error {
	if err := p.Page.Check(selector, opts); err != nil {
		ReportError(err, "xk6-playwright: error with checking the field")
		return err
	}
	return nil
}

// Uncheck wrapper around playwright uncheck page function that takes in a selector and a set of options
func (p *Playwright) Uncheck(selector string, opts playwright.FrameUncheckOptions) error {
	if err := p.Page.Uncheck(selector, opts); err != nil {
		ReportError(err, "xk6-playwright: error with unchecking the field")
		return err
	}
	return nil
}

// DragAndDrop wrapper around playwright draganddrop page function that takes in two selectors(source and target) and a set of options
func (p *Playwright) DragAndDrop(sourceSelector string, targetSelector string, opts playwright.FrameDragAndDropOptions) error {
	if err := p.Page.DragAndDrop(sourceSelector, targetSelector, opts); err != nil {
		ReportError(err, "xk6-playwright: error with dragging and dropping")
		return err
	}
	return nil
}

// Evaluate wrapper around playwright evaluate page function that takes in an expresion and a set of options and evaluates the expression/function returning the resulting information.
func (p *Playwright) Evaluate(expression string, opts playwright.PageEvaluateOptions) interface{} {
	returnedValue, err := p.Page.Evaluate(expression, opts)
	if err != nil {
		ReportError(err, "xk6-playwright: error with evaluating the expression")
		return nil
	}
	return returnedValue
}

// Reload wrapper around playwright reload page function
func (p *Playwright) Reload() error {
	if _, err := p.Page.Reload(); err != nil {
		ReportError(err, "xk6-playwright: error when reloading the page")
		return err
	}
	return nil
}

// FirstPaint function that gathers the Real User Monitoring Metrics for First Paint of the current page
func (p *Playwright) FirstPaint() uint64 {
	entries, err := p.Page.Evaluate("JSON.stringify(performance.getEntriesByName('first-paint'))")
	if err != nil {
		ReportError(err, "xk6-playwright: error with getting the first-paint entries")
		return 0
	}
	entriesToString := fmt.Sprintf("%v", entries)
	return gjson.Get(entriesToString, "0.startTime").Uint()
}

// FirstContentfulPaint function that gathers the Real User Monitoring Metrics for First Contentful Paint of the current page
func (p *Playwright) FirstContentfulPaint() uint64 {
	entries, err := p.Page.Evaluate("JSON.stringify(performance.getEntriesByName('first-contentful-paint'))")
	if err != nil {
		ReportError(err, "xk6-playwright: error with getting the first-contentful-paint entries")
		return 0
	}
	entriesToString := fmt.Sprintf("%v", entries)
	return gjson.Get(entriesToString, "0.startTime").Uint()
}

// TimeToMinimallyInteractive function that gathers the Real User Monitoring Metrics for Time to Minimally Interactive of the current page (based on the first input)
func (p *Playwright) TimeToMinimallyInteractive() uint64 {
	entries, err := p.Page.Evaluate("JSON.stringify(performance.getEntriesByType('first-input'))")
	if err != nil {
		ReportError(err, "xk6-playwright: error with getting the first-input entries for time to minimally interactive metrics")
		return 0
	}
	entriesToString := fmt.Sprintf("%v", entries)
	return gjson.Get(entriesToString, "0.startTime").Uint()
}

// FirstInputDelay function that gathers the Real User Monitoring Metrics for First Input Delay of the current page
func (p *Playwright) FirstInputDelay() uint64 {
	entries, err := p.Page.Evaluate("JSON.stringify(performance.getEntriesByType('first-input'))")
	if err != nil {
		ReportError(err, "xk6-playwright: error with getting the first-input entries for first input delay metrics")
		return 0
	}
	entriesToString := fmt.Sprintf("%v", entries)
	return gjson.Get(entriesToString, "0.processingStart").Uint() - gjson.Get(entriesToString, "0.startTime").Uint() //https://web.dev/fid/  for calc
}

// Cookies wrapper around playwright cookies fetch function
func (p *Playwright) Cookies() []*playwright.BrowserContextCookiesResult {
	cookies, err := p.cookies()
	if err != nil {
		ReportError(err, "xk6-playwright: error with getting the cookies")
		return nil
	}
	return cookies
}

//---------------------------------------------------------------------
//                         Helpers
//---------------------------------------------------------------------

// newPage creates a new page and returns it either with or without a context
func (p *Playwright) newPage() (playwright.Page, error) {
	if p.Browser != nil {
		return p.Browser.NewPage()
	}
	if p.BrowserContext != nil {
		return p.BrowserContext.NewPage()
	}
	return nil, errors.New("no browser or browser context attached")
}

// closeBrowser closes the browser and the browser context
func (p *Playwright) closeBrowser() error {
	if p.Browser != nil {
		return p.Browser.Close()
	}
	if p.BrowserContext != nil {
		return p.BrowserContext.Close()
	}
	return errors.New("no browser or browser context attached")
}

// cookies returns the cookies from the browser context or from browser persistent context
func (p *Playwright) cookies() ([]*playwright.BrowserContextCookiesResult, error) {
	if p.Browser != nil && len(p.Browser.Contexts()) > 0 {
		return p.Browser.Contexts()[0].Cookies()
	}
	if p.BrowserContext != nil {
		return p.BrowserContext.Cookies()
	}
	return nil, errors.New("no browser or browser context attached")
}

// ReportError reports an error if it is not nil
func ReportError(err error, msg string) {
	if err != nil {
		fmt.Printf("%s: %s", msg, err)
	}
}
