package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// pickerEntry is one row in the destination picker UI.
// isHeader rows are non-selectable category headings shown above their URLs.
type pickerEntry struct {
	label    string
	isHeader bool
	parent   *Destination
	url      DestinationURL
}

// filterDestinations returns the subset of dests whose Tag (or any child URL
// Tag) is present in tags. When tags is empty, or when any tag equals "all",
// all destinations are returned unchanged.
func filterDestinations(dests []Destination, tags []string) []Destination {
	if len(tags) == 0 {
		return dests
	}
	for _, t := range tags {
		if strings.ToLower(t) == "all" {
			return dests
		}
	}
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[strings.ToLower(t)] = true
	}
	var result []Destination
	for _, d := range dests {
		// Category tag matches → include the whole category.
		if d.Tag != "" && tagSet[strings.ToLower(d.Tag)] {
			result = append(result, d)
			continue
		}
		// Otherwise keep only the URL children whose tag matches.
		var matched []DestinationURL
		for _, u := range d.URLs {
			if u.Tag != "" && tagSet[strings.ToLower(u.Tag)] {
				matched = append(matched, u)
			}
		}
		if len(matched) > 0 {
			dc := d
			dc.URLs = matched
			result = append(result, dc)
		}
	}
	return result
}

// buildPickerEntries flattens filtered destinations into a flat display list.
// A category with no child URLs is a single selectable item.
// A category with child URLs gets a non-selectable header row followed by one
// entry per URL.
func buildPickerEntries(dests []Destination) []pickerEntry {
	var entries []pickerEntry
	for di := range dests {
		d := &dests[di]
		if len(d.URLs) == 0 {
			// Treat the whole category as one destination.
			entries = append(entries, pickerEntry{
				label:  d.Name,
				parent: d,
				url: DestinationURL{
					Label:                        d.Name,
					UsernameSelector:             d.UsernameSelector,
					UsernameValue:                d.UsernameValue,
					PasswordSelector:             d.PasswordSelector,
					PasswordValue:                d.PasswordValue,
					TOTPSecret:                   d.TOTPSecret,
					TOTPSelector:                 d.TOTPSelector,
					TOTPStep:                     d.TOTPStep,
					SubmitSelector:               d.SubmitSelector,
					TOTPSubmitSelector:           d.TOTPSubmitSelector,
					DoneSelector:                 d.DoneSelector,
					IgnoreCertErrors:             d.IgnoreCertErrors,
					Kiosk:                        d.Kiosk,
					KioskCloseButton:             d.KioskCloseButton,
					KioskCloseButtonLabel:        d.KioskCloseButtonLabel,
					KioskCloseButtonPosition:     d.KioskCloseButtonPosition,
					KioskCloseButtonSwapPosition: d.KioskCloseButtonSwapPosition,
					BrowserControls:              d.BrowserControls,
					BrowserControlsPosition:      d.BrowserControlsPosition,
					BrowserControlsSwapPosition:  d.BrowserControlsSwapPosition,
					StartMaximized:               d.StartMaximized,
					AppMode:                      d.AppMode,
					Incognito:                    d.Incognito,
					DisableContextMenu:           d.DisableContextMenu,
					DisableDevTools:              d.DisableDevTools,
					DisableTranslate:             d.DisableTranslate,
					DisablePinch:                 d.DisablePinch,
					DisableTouchAdjustment:       d.DisableTouchAdjustment,
					KioskPrinting:                d.KioskPrinting,
					NoFirstRun:                   d.NoFirstRun,
					NoDefaultBrowserCheck:        d.NoDefaultBrowserCheck,
				},
			})
		} else {
			entries = append(entries, pickerEntry{label: d.Name, isHeader: true})
			for ui := range d.URLs {
				u := &d.URLs[ui]
				label := u.Label
				if label == "" {
					label = u.AuthStartURL
				}
				entries = append(entries, pickerEntry{label: label, parent: d, url: *u})
			}
		}
	}
	return entries
}

// selectDestinationHeadless presents a numbered list on stdout and reads a
// choice from stdin. Used in headless (no Fyne GUI) mode.
func selectDestinationHeadless(entries []pickerEntry) (DestinationURL, *Destination, error) {
	var selectable []pickerEntry

	fmt.Fprintln(os.Stdout, "\n── Select destination ──")
	idx := 1
	for _, e := range entries {
		if e.isHeader {
			fmt.Fprintf(os.Stdout, "\n  %s\n", e.label)
			continue
		}
		fmt.Fprintf(os.Stdout, "    [%d] %s\n", idx, e.label)
		selectable = append(selectable, e)
		idx++
	}
	fmt.Fprintln(os.Stdout, "")

	if len(selectable) == 0 {
		return DestinationURL{}, nil, fmt.Errorf("no selectable destinations available")
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprintf(os.Stdout, "Select [1-%d]: ", len(selectable))
		line, _ := reader.ReadString('\n')
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || n < 1 || n > len(selectable) {
			fmt.Fprintf(os.Stdout, "Please enter a number between 1 and %d.\n", len(selectable))
			continue
		}
		e := selectable[n-1]
		return e.url, e.parent, nil
	}
}

// singleSelectableEntry returns the one non-header entry when the list has
// exactly one selectable item so the picker can be skipped automatically.
func singleSelectableEntry(entries []pickerEntry) (pickerEntry, bool) {
	var found []pickerEntry
	for _, e := range entries {
		if !e.isHeader {
			found = append(found, e)
		}
	}
	if len(found) == 1 {
		return found[0], true
	}
	return pickerEntry{}, false
}

// applyDestination merges a chosen DestinationURL and its parent Destination
// into the root Config. Precedence (highest wins):
//
//	URL-level field > category-level field > existing root Config field.
func applyDestination(cfg *Config, parent *Destination, chosen DestinationURL) {
	applyNonEmpty := func(dst *string, catVal, urlVal string) {
		if catVal != "" {
			*dst = catVal
		}
		if urlVal != "" {
			*dst = urlVal
		}
	}
	applyNonEmptyInt := func(dst *int, catVal, urlVal int) {
		if catVal != 0 {
			*dst = catVal
		}
		if urlVal != 0 {
			*dst = urlVal
		}
	}
	applyBoolPtr := func(dst *bool, catVal, urlVal *bool) {
		if catVal != nil {
			*dst = *catVal
		}
		if urlVal != nil {
			*dst = *urlVal
		}
	}
	applyBoolPtrValue := func(dst **bool, catVal, urlVal *bool) {
		if catVal != nil {
			v := *catVal
			*dst = &v
		}
		if urlVal != nil {
			v := *urlVal
			*dst = &v
		}
	}
	if parent != nil {
		applyNonEmpty(&cfg.UsernameSelector, parent.UsernameSelector, chosen.UsernameSelector)
		applyNonEmpty(&cfg.UsernameValue, parent.UsernameValue, chosen.UsernameValue)
		applyNonEmpty(&cfg.PasswordSelector, parent.PasswordSelector, chosen.PasswordSelector)
		applyNonEmpty(&cfg.PasswordValue, parent.PasswordValue, chosen.PasswordValue)
		applyNonEmpty(&cfg.TOTPSecret, parent.TOTPSecret, chosen.TOTPSecret)
		applyNonEmpty(&cfg.TOTPSelector, parent.TOTPSelector, chosen.TOTPSelector)
		applyNonEmptyInt(&cfg.TOTPStep, parent.TOTPStep, chosen.TOTPStep)
		applyNonEmpty(&cfg.SubmitSelector, parent.SubmitSelector, chosen.SubmitSelector)
		applyNonEmpty(&cfg.TOTPSubmitSelector, parent.TOTPSubmitSelector, chosen.TOTPSubmitSelector)
		applyNonEmpty(&cfg.DoneSelector, parent.DoneSelector, chosen.DoneSelector)
		applyNonEmptyInt(&cfg.WaitAfterSubmitMs, parent.WaitAfterSubmitMs, chosen.WaitAfterSubmitMs)
		applyBoolPtr(&cfg.IgnoreCertErrors, parent.IgnoreCertErrors, chosen.IgnoreCertErrors)
		applyBoolPtr(&cfg.Kiosk, parent.Kiosk, chosen.Kiosk)
		applyBoolPtrValue(&cfg.KioskCloseButton, parent.KioskCloseButton, chosen.KioskCloseButton)
		applyNonEmpty(&cfg.KioskCloseButtonLabel, parent.KioskCloseButtonLabel, chosen.KioskCloseButtonLabel)
		applyNonEmpty(&cfg.KioskCloseButtonPosition, parent.KioskCloseButtonPosition, chosen.KioskCloseButtonPosition)
		applyBoolPtr(&cfg.KioskCloseButtonSwapPosition, parent.KioskCloseButtonSwapPosition, chosen.KioskCloseButtonSwapPosition)
		applyBoolPtr(&cfg.BrowserControls, parent.BrowserControls, chosen.BrowserControls)
		applyNonEmpty(&cfg.BrowserControlsPosition, parent.BrowserControlsPosition, chosen.BrowserControlsPosition)
		applyBoolPtr(&cfg.BrowserControlsSwapPosition, parent.BrowserControlsSwapPosition, chosen.BrowserControlsSwapPosition)
		applyBoolPtr(&cfg.StartMaximized, parent.StartMaximized, chosen.StartMaximized)
		applyBoolPtr(&cfg.AppMode, parent.AppMode, chosen.AppMode)
		applyBoolPtr(&cfg.Incognito, parent.Incognito, chosen.Incognito)
		applyBoolPtr(&cfg.DisableContextMenu, parent.DisableContextMenu, chosen.DisableContextMenu)
		applyBoolPtr(&cfg.DisableDevTools, parent.DisableDevTools, chosen.DisableDevTools)
		applyBoolPtr(&cfg.DisableTranslate, parent.DisableTranslate, chosen.DisableTranslate)
		applyBoolPtr(&cfg.DisablePinch, parent.DisablePinch, chosen.DisablePinch)
		applyBoolPtr(&cfg.DisableTouchAdjustment, parent.DisableTouchAdjustment, chosen.DisableTouchAdjustment)
		applyBoolPtr(&cfg.KioskPrinting, parent.KioskPrinting, chosen.KioskPrinting)
		applyBoolPtr(&cfg.NoFirstRun, parent.NoFirstRun, chosen.NoFirstRun)
		applyBoolPtr(&cfg.NoDefaultBrowserCheck, parent.NoDefaultBrowserCheck, chosen.NoDefaultBrowserCheck)
	}
	if chosen.AuthStartURL != "" {
		cfg.AuthStartURL = chosen.AuthStartURL
	}
	if chosen.AuthDoneURL != "" {
		cfg.AuthDoneURL = chosen.AuthDoneURL
	}
	if chosen.NavigateURL != "" {
		cfg.NavigateURL = chosen.NavigateURL
	}
}
