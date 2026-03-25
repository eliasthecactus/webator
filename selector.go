package main

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/chromedp/chromedp"
)

// selectorType distinguishes how a selector expression should be evaluated.
type selectorType int

const (
	selectorCSS   selectorType = iota
	selectorXPath selectorType = iota
)

// resolvedSelector bundles a selector string with its type and the chromedp
// QueryOption that matches that type.
type resolvedSelector struct {
	raw      string
	stype    selectorType
	byOption chromedp.QueryOption
}

// resolveSelector inspects the selector string and decides whether it is an
// XPath expression or a CSS selector, returning the appropriate resolved form.
//
// Rules:
//   - Starts with "//" or ".//" → XPath, chromedp.BySearch
//   - Starts with "#"           → CSS id, chromedp.ByCSSSelector
//   - Anything else             → CSS, chromedp.ByCSSSelector
func resolveSelector(sel string, logger *slog.Logger) resolvedSelector {
	var rs resolvedSelector
	rs.raw = sel

	switch {
	case strings.HasPrefix(sel, "//") || strings.HasPrefix(sel, ".//"):
		rs.stype = selectorXPath
		rs.byOption = chromedp.BySearch
		logger.Debug("resolved selector as XPath", slog.String("selector", sel))
	default:
		rs.stype = selectorCSS
		rs.byOption = chromedp.ByQuery
		logger.Debug("resolved selector as CSS", slog.String("selector", sel))
	}

	return rs
}

// visibilityJS returns a self-contained JavaScript snippet that returns true
// when the element identified by rs.raw is visible and enabled, false
// otherwise.  The snippet is safe against missing elements and JS errors.
func (rs resolvedSelector) visibilityJS() string {
	// Safely embed the selector into JS using JSON encoding.
	selJSON, err := json.Marshal(rs.raw)
	if err != nil {
		// Fallback: wrap in double quotes after escaping; should never happen
		// because json.Marshal on a plain string only fails on invalid UTF-8.
		selJSON = []byte(`"` + strings.ReplaceAll(rs.raw, `"`, `\"`) + `"`)
	}
	selStr := string(selJSON)

	switch rs.stype {
	case selectorXPath:
		return `(function(){
  try {
    var sel = ` + selStr + `;
    var result = document.evaluate(sel, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
    var el = result.singleNodeValue;
    if (!el) return false;
    var style = window.getComputedStyle(el);
    if (style.display === 'none') return false;
    if (style.visibility === 'hidden') return false;
    var rect = el.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0) return false;
    if (el.disabled) return false;
    return true;
  } catch(e) {
    return false;
  }
})()`

	default: // selectorCSS
		return `(function(){
  try {
    var sel = ` + selStr + `;
    var el = document.querySelector(sel);
    if (!el) return false;
    var style = window.getComputedStyle(el);
    if (style.display === 'none') return false;
    if (style.visibility === 'hidden') return false;
    var rect = el.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0) return false;
    if (el.disabled) return false;
    return true;
  } catch(e) {
    return false;
  }
})()`
	}
}
