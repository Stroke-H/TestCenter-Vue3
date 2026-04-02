package services

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/playwright-community/playwright-go"
)

// InspectorMessage defines the structure for messages between Inspector and Frontend
type InspectorMessage struct {
	Type     string `json:"type"`     // "status", "element_picked", "log", "error"
	Selector string `json:"selector"` // The CSS selector of the picked element
	Message  string `json:"message"`  // Status or error message
	URL      string `json:"url"`      // Current browser URL
	Value    string `json:"value"`    // Recorded input value
}

// ServeInspectorWS handles the WebSocket connection for the UI Inspector
func ServeInspectorWS(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Inspector WebSocket Upgrade Error:", err)
		return
	}
	defer ws.Close()

	targetURL := c.Query("url")
	if targetURL == "" {
		targetURL = "about:blank"
	}

	// Initialize Playwright
	pw, err := playwright.Run()
	if err != nil {
		sendInspectorError(ws, fmt.Sprintf("Failed to run playwright: %v", err))
		return
	}
	defer pw.Stop()

	// Launch headful browser
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false),
	})
	if err != nil {
		sendInspectorError(ws, fmt.Sprintf("Failed to launch browser: %v", err))
		return
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		sendInspectorError(ws, fmt.Sprintf("Failed to create page: %v", err))
		return
	}

	// Relate console messages to backend logs
	page.On("console", func(msg playwright.ConsoleMessage) {
		log.Printf("[Browser Console] %s: %s", msg.Type(), msg.Text())
	})

	// Expose function to bridge JS -> Go
	err = page.ExposeFunction("sendSelectorToTestCenter", func(args ...interface{}) interface{} {
		log.Printf("Selector received in Go: %v", args)
		if len(args) > 0 {
			switch payload := args[0].(type) {
			case string:
				msg := InspectorMessage{
					Type:     "element_picked",
					Selector: payload,
					URL:      page.URL(),
				}
				_ = ws.WriteJSON(msg)
			case map[string]interface{}:
				msg := InspectorMessage{
					Type:     getInspectorString(payload["type"], "element_picked"),
					Selector: getInspectorString(payload["selector"], ""),
					Message:  getInspectorString(payload["message"], ""),
					URL:      getInspectorString(payload["url"], page.URL()),
					Value:    getInspectorString(payload["value"], ""),
				}
				_ = ws.WriteJSON(msg)
			default:
				if raw, err := json.Marshal(payload); err == nil {
					var decoded InspectorMessage
					if err := json.Unmarshal(raw, &decoded); err == nil {
						if decoded.Type == "" {
							decoded.Type = "element_picked"
						}
						if decoded.URL == "" {
							decoded.URL = page.URL()
						}
						_ = ws.WriteJSON(decoded)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		sendInspectorError(ws, fmt.Sprintf("Failed to expose function: %v", err))
		return
	}

	// Inject Inspector Script
	script := inspectorScript
	err = page.AddInitScript(playwright.Script{
		Content: &script,
	})
	if err != nil {
		sendInspectorError(ws, fmt.Sprintf("Failed to inject script: %v", err))
		return
	}

	// Navigate to target
	_, err = page.Goto(targetURL)
	if err != nil {
		sendInspectorError(ws, fmt.Sprintf("Failed to navigate: %v", err))
	}

	// Fallback: Manually evaluate script after navigation just in case AddInitScript missed it
	_, _ = page.Evaluate(inspectorScript)

	sendInspectorStatus(ws, "Inspector started and injected")

	// Wait for browser close or WS disconnect
	done := make(chan bool)

	// Monitor browser close
	browser.On("disconnected", func() {
		log.Println("Browser disconnected")
		done <- true
	})

	// Monitor WS close/messages
	go func() {
		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				log.Println("WS Disconnected")
				done <- true
				return
			}
		}
	}()

	<-done
	log.Println("Inspector session ended")
}

func sendInspectorError(ws *websocket.Conn, msg string) {
	_ = ws.WriteJSON(InspectorMessage{
		Type:    "error",
		Message: msg,
	})
}

func sendInspectorStatus(ws *websocket.Conn, msg string) {
	_ = ws.WriteJSON(InspectorMessage{
		Type:    "status",
		Message: msg,
	})
}

func getInspectorString(value interface{}, fallback string) string {
	if str, ok := value.(string); ok {
		return str
	}
	return fallback
}

// inspectorScript is the JS injected into the target page
const inspectorScript = `
(function() {
  console.log('TestCenter Inspector Injected');
  
  const style = document.createElement('style');
  style.innerHTML = '.tc-inspector-highlight { outline: 2px solid #409eff !important; outline-offset: -2px !important; background-color: rgba(64, 158, 255, 0.1) !important; cursor: crosshair !important; transition: all 0.1s ease !important; z-index: 2147483647 !important; }';
  document.head.appendChild(style);

  let lastElement = null;
  const inputTimers = new WeakMap();

  document.addEventListener('mouseover', (e) => {
    if (lastElement) {
      lastElement.classList.remove('tc-inspector-highlight');
    }
    lastElement = e.target;
    if (lastElement && lastElement.classList) {
      lastElement.classList.add('tc-inspector-highlight');
    }
  }, true);

  document.addEventListener('click', (e) => {
    const target = getActionableTarget(e.target);
    const selector = getOptimalSelector(target);
    console.log('Element Clicked, Selector:', selector);
    
    if (window.sendSelectorToTestCenter) {
      window.sendSelectorToTestCenter({
        type: 'element_picked',
        selector,
        url: window.location.href
      });
      // Visual feedback in the inspected page
      const originalBg = target.style.backgroundColor;
      target.style.backgroundColor = 'rgba(103, 194, 58, 0.5)';
      setTimeout(() => { target.style.backgroundColor = originalBg; }, 500);
    } else {
      console.error('sendSelectorToTestCenter not found!');
    }
  }, true);

  document.addEventListener('keydown', (e) => {
    if (!shouldRecordKeydown(e)) return;

    const target = e.target;
    const selector = getOptimalSelector(target);
    if (!selector) return;

    console.log('Key Pressed, Selector:', selector, 'Key:', e.key);
    if (window.sendSelectorToTestCenter) {
      window.sendSelectorToTestCenter({
        type: 'element_keydown',
        selector,
        value: e.key,
        url: window.location.href
      });
    }
  }, true);

  document.addEventListener('input', (e) => {
    const target = e.target;
    if (!isFillableElement(target)) return;

    const existingTimer = inputTimers.get(target);
    if (existingTimer) {
      clearTimeout(existingTimer);
    }

    const timer = setTimeout(() => {
      const selector = getOptimalSelector(target);
      const value = target.value || '';
      if (!selector) return;

      console.log('Element Filled, Selector:', selector, 'Value:', value);
      if (window.sendSelectorToTestCenter) {
        window.sendSelectorToTestCenter({
          type: 'element_filled',
          selector,
          value,
          url: window.location.href
        });
      }
    }, 500);

    inputTimers.set(target, timer);
  }, true);

  function isFillableElement(el) {
    return el &&
      el.nodeType === Node.ELEMENT_NODE &&
      (
        el.matches('input:not([type="checkbox"]):not([type="radio"]):not([type="file"]):not([type="submit"]):not([type="button"])') ||
        el.matches('textarea') ||
        el.isContentEditable
      );
  }

  function shouldRecordKeydown(e) {
    if (!e || !e.key) return false;
    if (e.isComposing) return false;
    return ['Enter', 'Tab', 'Escape'].includes(e.key);
  }

  function getActionableTarget(el) {
    return el.closest('button, a, input[type="button"], input[type="submit"], [role="button"]') || el;
  }

  function escapeValue(value) {
    if (window.CSS && typeof window.CSS.escape === 'function') {
      return window.CSS.escape(value);
    }
    return String(value).replace(/["\\]/g, '\\$&');
  }

  function unique(selector) {
    try {
      return !!selector && document.querySelectorAll(selector).length === 1;
    } catch (e) {
      return false;
    }
  }

  function getOptimalSelector(el) {
    if (!el || el.nodeType !== Node.ELEMENT_NODE) return '';

    if (el.id) {
      const selector = '#' + escapeValue(el.id);
      if (unique(selector)) return selector;
    }

    const tag = el.nodeName.toLowerCase();
    const stableAttrs = [
      ['data-testid', el.getAttribute('data-testid')],
      ['data-test-id', el.getAttribute('data-test-id')],
      ['name', el.getAttribute('name')],
      ['aria-label', el.getAttribute('aria-label')],
      ['placeholder', el.getAttribute('placeholder')],
      ['title', el.getAttribute('title')],
      ['value', el.getAttribute('value')]
    ];

    for (const [attr, value] of stableAttrs) {
      if (!value) continue;
      const selector = tag + '[' + attr + '="' + escapeValue(value) + '"]';
      if (unique(selector)) return selector;
    }

    const role = el.getAttribute('role');
    const ariaLabel = el.getAttribute('aria-label');
    if (role && ariaLabel) {
      const selector = '[role="' + escapeValue(role) + '"][aria-label="' + escapeValue(ariaLabel) + '"]';
      if (unique(selector)) return selector;
    }

    const classList = Array.from(el.classList || []).filter(cls => cls && !cls.startsWith('tc-inspector'));
    if (classList.length > 0) {
      const selector = tag + '.' + classList.slice(0, 2).map(escapeValue).join('.');
      if (unique(selector)) return selector;
    }

    const parts = [];
    while (el && el.nodeType === Node.ELEMENT_NODE) {
      let selector = el.nodeName.toLowerCase();
      if (el.id) {
        selector += '#' + escapeValue(el.id);
        parts.unshift(selector);
        break;
      } else {
        let sib = el, nth = 1;
        while (sib = sib.previousElementSibling) {
          if (sib.nodeName.toLowerCase() == selector) nth++;
        }
        if (nth != 1) selector += ":nth-of-type(" + nth + ")";
      }
      parts.unshift(selector);
      el = el.parentElement;
    }
    return parts.join(' > ');
  }
})();
`
