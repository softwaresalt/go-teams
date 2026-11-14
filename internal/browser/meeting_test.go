package browser

import (
	"context"
	"io"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
)

// TestMeetingInputValidation covers the input guards on the Meeting methods that
// do not require a live browser: nil sink/writer and a non-chromedp context are
// all rejected.
func TestMeetingInputValidation(t *testing.T) {
	m := NewMeeting(MeetingOptions{MeetingURL: "https://teams.microsoft.com/x", BotName: "Agent"})
	bg := context.Background()

	assert.Error(t, m.EnableCaptions(bg, nil), "nil caption sink")
	assert.Error(t, m.EnableCaptions(bg, func(string, string) {}), "non-chromedp context")
	assert.Error(t, m.CaptureAudio(bg, nil), "nil audio writer")
	assert.Error(t, m.CaptureAudio(bg, io.Discard), "non-chromedp context")
}

// TestNewLiveMeeting verifies the live-meeting constructor wires a Meeting,
// context, and cleanup func without launching a browser (Chromium starts only
// on the first chromedp.Run). Cleanup must be safe to call regardless.
func TestNewLiveMeeting(t *testing.T) {
	m, ctx, cleanup, err := NewLiveMeeting(context.Background(),
		Options{Headless: true},
		MeetingOptions{MeetingURL: "https://teams.microsoft.com/x", BotName: "Agent"})
	assert.NoError(t, err)
	assert.NotNil(t, m)
	assert.NotNil(t, ctx)
	if assert.NotNil(t, cleanup) {
		cleanup()
	}
}

func TestTargetContextPropagatesCallerCancellationWithAttachedCtx(t *testing.T) {
	allocCtx, cancelAlloc := context.WithCancel(context.Background())
	defer cancelAlloc()

	cm := newChromeMeeting(MeetingOptions{MeetingURL: "https://teams.microsoft.com/x"})
	cm.attachedCtx = allocCtx

	callerCtx, cancelCaller := context.WithCancel(context.Background())
	targetCtx, cancelTarget := cm.targetContext(callerCtx)
	defer cancelTarget()

	cancelCaller()
	select {
	case <-targetCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("targetCtx was not cancelled after callerCtx cancellation")
	}
	assert.ErrorIs(t, targetCtx.Err(), context.Canceled,
		"cancelling callerCtx must cancel targetCtx even when m.attachedCtx is still active")
	assert.NoError(t, allocCtx.Err(),
		"cancelling callerCtx must not cancel the underlying attachedCtx")
}

func TestNewLiveMeetingPropagatesDefaultAccount(t *testing.T) {
	m, _, cleanup, err := NewLiveMeeting(context.Background(),
		Options{Headless: true, DefaultAccount: "  alice@contoso.com  "},
		MeetingOptions{MeetingURL: "https://teams.microsoft.com/x", BotName: "Agent"})
	assert.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}
	cm, ok := m.(*chromeMeeting)
	if assert.True(t, ok) {
		assert.Equal(t, "alice@contoso.com", cm.opts.DefaultAccount)
		assert.Equal(t, "alice@contoso.com", cm.defaultAccount())
	}

	// Explicit MeetingOptions.DefaultAccount takes precedence over Options.DefaultAccount.
	m2, _, cleanup2, err := NewLiveMeeting(context.Background(),
		Options{Headless: false, DefaultAccount: "fallback@contoso.com"},
		MeetingOptions{MeetingURL: "https://teams.microsoft.com/x", BotName: "Agent", DefaultAccount: "explicit@contoso.com"})
	assert.NoError(t, err)
	if cleanup2 != nil {
		defer cleanup2()
	}
	cm2, ok := m2.(*chromeMeeting)
	if assert.True(t, ok) {
		assert.Equal(t, "explicit@contoso.com", cm2.defaultAccount())
	}
}

func TestSelectEntraAccountJSDOM(t *testing.T) {
	js := selectEntraAccountJS("alice@contoso.com")
	if !assert.NotEmpty(t, js) || !assert.NotEqual(t, "false", js) {
		return
	}
	execPath := ResolveExecPath("")
	if runtime.GOOS != "windows" {
		if _, err := os.Stat("/usr/bin/google-chrome"); err == nil {
			execPath = "/usr/bin/google-chrome"
		}
	}
	if execPath == "" {
		t.Skip("no Chromium-based browser executable found on host")
	}

	opts := Options{
		Headless:       true,
		UserDataDir:    t.TempDir(),
		ExecPath:       execPath,
		DisableSandbox: runtime.GOOS != "windows",
	}
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), cleanHeadlessAllocatorOptions(opts)...)
	defer cancelAlloc()
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	const pickerHTML = `<!doctype html><html><body>
		<div id="tilesHolder">
			<div class="table" role="button" data-test-id="bob.alice@contoso.com" onclick="window.__clicked='bob.alice@contoso.com'">
				<div class="table-cell">Bob Alice</div>
				<small>bob.alice@contoso.com</small>
			</div>
			<div class="table" role="button" data-test-id="Alice@Contoso.com" onclick="window.__clicked='alice@contoso.com'">
				<div class="table-cell">Alice Smith</div>
				<small>Alice@Contoso.com</small>
			</div>
			<div class="table" role="button" id="otherTile" data-test-id="otherTile" onclick="window.__clicked='otherTile'">
				<div class="table-cell">Use another account</div>
			</div>
		</div>
	</body></html>`

	if err := chromedp.Run(ctx, chromedp.Navigate("about:blank")); err != nil {
		t.Fatalf("navigate about:blank: %v", err)
	}
	setBody := func(html string) {
		t.Helper()
		var ok bool
		err := chromedp.Run(ctx, chromedp.Evaluate(`(() => { document.body.innerHTML = `+"`"+html+"`"+`; window.__clicked = ""; return true; })()`, &ok))
		assert.NoError(t, err)
	}

	// 1. Exact case-insensitive UPN match clicks Alice@Contoso.com (not bob.alice@contoso.com or #otherTile).
	setBody(pickerHTML)
	var clicked bool
	assert.NoError(t, chromedp.Run(ctx, chromedp.Evaluate(selectEntraAccountJS("ALICE@CONTOSO.COM"), &clicked)))
	assert.True(t, clicked, "expected matching UPN tile to be clicked")
	var clickedID string
	assert.NoError(t, chromedp.Run(ctx, chromedp.Evaluate(`window.__clicked || ""`, &clickedID)))
	assert.Equal(t, "alice@contoso.com", clickedID)

	// 2. Cooldown guard prevents re-clicking within 3000ms.
	assert.NoError(t, chromedp.Run(ctx, chromedp.Evaluate(`window.__clicked = ""; true`, &clicked)))
	assert.NoError(t, chromedp.Run(ctx, chromedp.Evaluate(selectEntraAccountJS("alice@contoso.com"), &clicked)))
	assert.False(t, clicked, "expected 3000ms cooldown guard to prevent immediate re-click")

	// 3. Exact display-name match works, while partial prefix "John" does not match "Johnathan".
	setBody(`<div id="tilesHolder">
		<div class="table" role="button" onclick="window.__clicked='johnathan'"><div class="table-cell">Johnathan Doe</div></div>
		<div class="table" role="button" onclick="window.__clicked='alice-display'"><div class="table-cell">Alice Smith</div></div>
	</div>`)
	assert.NoError(t, chromedp.Run(ctx, chromedp.Evaluate(selectEntraAccountJS("John"), &clicked)))
	assert.False(t, clicked, "partial display name prefix must not match")
	assert.NoError(t, chromedp.Run(ctx, chromedp.Evaluate(selectEntraAccountJS("alice smith"), &clicked)))
	assert.True(t, clicked, "exact display name should match case-insensitively")
	assert.NoError(t, chromedp.Run(ctx, chromedp.Evaluate(`window.__clicked || ""`, &clickedID)))
	assert.Equal(t, "alice-display", clickedID)

	// 4. Ambiguous match (>1 tiles matching the same display name) fails closed without clicking.
	setBody(`<div id="tilesHolder">
		<div class="table" role="button" onclick="window.__clicked='dup1'"><div class="table-cell">Shared Name</div></div>
		<div class="table" role="button" onclick="window.__clicked='dup2'"><div class="table-cell">Shared Name</div></div>
	</div>`)
	assert.NoError(t, chromedp.Run(ctx, chromedp.Evaluate(selectEntraAccountJS("Shared Name"), &clicked)))
	assert.False(t, clicked, "ambiguous match must fail closed")

	// 5. Attempting to match "otherTile" or "Use another account" never clicks #otherTile.
	setBody(pickerHTML)
	assert.NoError(t, chromedp.Run(ctx, chromedp.Evaluate(selectEntraAccountJS("otherTile"), &clicked)))
	assert.False(t, clicked, "#otherTile must always be skipped")

	// 6. Visible password input (even if #tilesHolder remains in DOM) returns false without clicking.
	setBody(`<div id="tilesHolder"><input type="password" style="width:120px;height:32px;display:block" /><div class="table" role="button" data-test-id="alice@contoso.com" onclick="window.__clicked='pwd-page'">Alice@Contoso.com</div></div>`)
	assert.NoError(t, chromedp.Run(ctx, chromedp.Evaluate(selectEntraAccountJS("alice@contoso.com"), &clicked)))
	assert.False(t, clicked, "visible password input must prevent clicking account tiles")

	// 7. Visible MFA one-time-code challenge input returns false without clicking.
	setBody(`<div id="tilesHolder"><input name="otc" autocomplete="one-time-code" style="width:120px;height:32px;display:block" /><div class="table" role="button" data-test-id="alice@contoso.com" onclick="window.__clicked='mfa-page'">Alice@Contoso.com</div></div>`)
	assert.NoError(t, chromedp.Run(ctx, chromedp.Evaluate(selectEntraAccountJS("alice@contoso.com"), &clicked)))
	assert.False(t, clicked, "visible MFA challenge input must prevent clicking account tiles")

	// 8. Tile beneath a display:none ancestor is ignored (neither clicked nor treated as an ambiguous duplicate).
	setBody(`<div id="tilesHolder">
		<div style="display:none">
			<div class="table" role="button" data-test-id="alice@contoso.com" onclick="window.__clicked='hidden-tile'">Alice@Contoso.com</div>
		</div>
		<div class="table" role="button" data-test-id="alice@contoso.com" onclick="window.__clicked='visible-tile'">Alice@Contoso.com</div>
	</div>`)
	assert.NoError(t, chromedp.Run(ctx, chromedp.Evaluate(selectEntraAccountJS("alice@contoso.com"), &clicked)))
	assert.True(t, clicked, "visible tile should be clicked while hidden-ancestor duplicate is ignored")
	assert.NoError(t, chromedp.Run(ctx, chromedp.Evaluate(`window.__clicked || ""`, &clickedID)))
	assert.Equal(t, "visible-tile", clickedID)

	// 9. Page without a Microsoft account-picker container does not click generic buttons matching account text.
	setBody(`<div class="table" role="button" onclick="window.__clicked='unrelated-btn'"><div class="table-cell">Alice Smith</div></div>`)
	assert.NoError(t, chromedp.Run(ctx, chromedp.Evaluate(selectEntraAccountJS("Alice Smith"), &clicked)))
	assert.False(t, clicked, "generic button outside an account-picker surface must not be clicked")
}
