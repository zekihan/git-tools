package main

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"
)

func TestCleanupV2Keys(t *testing.T) {
	model := cleanupModel{
		ctx:     t.Context(),
		entries: []cleanupEntry{{branch: "one"}, {branch: "two"}},
		preview: strings.Repeat("line\n", 20),
	}
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyDown}, {Code: 'j', Text: "j"}} {
		updated, cmd := model.Update(key)

		model = cleanupTestModel(t, updated)
		if model.cursor != 1 || model.offset != 0 || cmd == nil {
			t.Fatalf(
				"%s: navigation = cursor %d, offset %d, command %v",
				key,
				model.cursor,
				model.offset,
				cmd != nil,
			)
		}
	}

	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyUp}, {Code: 'k', Text: "k"}} {
		updated, cmd := model.Update(key)

		model = cleanupTestModel(t, updated)
		if model.cursor != 0 || cmd == nil {
			t.Fatalf("%s: navigation = cursor %d, command %v", key, model.cursor, cmd != nil)
		}
	}

	model.preview = strings.Repeat("line\n", 20)
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})

	model = cleanupTestModel(t, updated)
	if model.offset != tuiScrollStep {
		t.Fatalf("page down offset = %d", model.offset)
	}

	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})

	model = cleanupTestModel(t, updated)
	if model.offset != 0 {
		t.Fatalf("page up offset = %d", model.offset)
	}

	updated, cmd := model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})

	model = cleanupTestModel(t, updated)
	if !model.loading || cmd == nil {
		t.Fatal("refresh did not start a reload")
	}
}

func TestCleanupV2QuitAndKeyRelease(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{
		{Code: 'q', Text: "q"},
		{Code: tea.KeyEscape},
		{Code: 'c', Mod: tea.ModCtrl},
	} {
		model := cleanupModel{}

		_, cmd := model.Update(key)
		if cmd == nil {
			t.Fatalf("%s did not quit", key)
		}

		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%s did not return QuitMsg", key)
		}
	}

	model := cleanupModel{entries: []cleanupEntry{{branch: "one"}, {branch: "two"}}}
	for _, key := range []tea.KeyReleaseMsg{
		{Code: 'q', Text: "q"},
		{Code: 'd', Text: "d"},
		{Code: tea.KeyDown},
	} {
		updated, cmd := model.Update(key)

		model = cleanupTestModel(t, updated)
		if cmd != nil || model.confirm || model.cursor != 0 {
			t.Fatalf("key release %s triggered an action", key)
		}
	}
}

func TestCleanupV2ConfirmationInput(t *testing.T) {
	for _, input := range []tea.Msg{
		tea.PasteMsg{Content: "dEé\x1bele\nte extra"},
		tea.KeyPressMsg{Text: "dEé\x1bele\nte extra"},
	} {
		model := cleanupModel{forceConfirm: true, pending: cleanupEntry{branch: "feature"}}
		updated, cmd := model.Update(input)

		model = cleanupTestModel(t, updated)
		if model.typed != "delete" || !model.forceConfirm || cmd != nil {
			t.Fatalf(
				"%T: text = %q, confirmation = %v, command = %v",
				input,
				model.typed,
				model.forceConfirm,
				cmd != nil,
			)
		}

		updated, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})

		model = cleanupTestModel(t, updated)
		if model.typed != "delet" || cmd != nil {
			t.Fatalf("backspace = %q, command = %v", model.typed, cmd != nil)
		}

		updated, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		model = cleanupTestModel(t, updated)
		if model.busy || cmd != nil {
			t.Fatal("incomplete text started deletion")
		}

		updated, _ = model.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
		model = cleanupTestModel(t, updated)
		updated, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		model = cleanupTestModel(t, updated)
		if !model.busy || model.forceConfirm || cmd == nil {
			t.Fatal("complete text and enter did not start deletion")
		}
	}
}

func TestCleanupV2PasteDoesNotTriggerActions(t *testing.T) {
	for _, initial := range []cleanupModel{
		{},
		{confirm: true},
		{busy: true, confirm: true},
		{busy: true, forceConfirm: true},
	} {
		model := initial
		for _, text := range []string{"q", "d", "b", "y", "delete\n"} {
			updated, cmd := model.Update(tea.PasteMsg{Content: text})

			model = cleanupTestModel(t, updated)
			if cmd != nil || model.typed != "" || model.busy != initial.busy {
				t.Fatalf("paste %q triggered an action", text)
			}

			if !model.busy && model.confirm {
				t.Fatal("paste must cancel the initial confirmation, as in v1")
			}
		}
	}
}

func TestCleanupV2BackgroundAndResize(t *testing.T) {
	for _, background := range []color.Color{color.White, color.Black} {
		model := cleanupModel{width: 80, height: 24}
		updated, cmd := model.Update(tea.BackgroundColorMsg{Color: background})

		model = cleanupTestModel(t, updated)
		if model.lightBackground != (background == color.White) || cmd != nil {
			t.Fatalf(
				"background = %v, light = %v, command = %v",
				background,
				model.lightBackground,
				cmd != nil,
			)
		}

		updated, cmd = model.Update(tea.WindowSizeMsg{Width: 59, Height: 15})
		model = cleanupTestModel(t, updated)

		view := model.View()
		if model.width != 59 || model.height != 15 || cmd != nil || !view.AltScreen ||
			!strings.Contains(view.Content, "Resize terminal to at least 60x16") {
			t.Fatal("resize did not preserve the small-terminal view")
		}
	}
}

func TestCleanupBackgroundEnvironmentFallback(t *testing.T) {
	for env, wantLight := range map[string]bool{
		"": false, "15;0": false, "0;7": true, "0;15": true, "0;default;15": true,
		"15": false, "0;invalid": false, "0;-1": false, "0;256": false,
	} {
		if got := tuiLightBackground(env); got != wantLight {
			t.Errorf("COLORFGBG=%q: light = %v, want %v", env, got, wantLight)
		}
	}
}

func TestCleanupPaletteMatchesV1(t *testing.T) {
	for _, dark := range []bool{false, true} {
		styles := newTuiStyles(dark)

		colors := []struct {
			name  string
			color color.Color
			light string
			dark  string
		}{
			{"accent", styles.accent, "#2563eb", "#82aaff"},
			{"border", styles.border, "#cbd5e1", "#3b4961"},
			{"muted", styles.muted.GetForeground(), "#64748b", "#8291a8"},
			{"heading", styles.heading.GetForeground(), "#2563eb", "#82aaff"},
			{"selection text", styles.selected.GetForeground(), "#1d4ed8", "#dbeafe"},
			{"selection background", styles.selected.GetBackground(), "#e8f0fe", "#24344f"},
			{"added", styles.added.GetForeground(), "#15803d", "#91d7a3"},
			{"removed", styles.removed.GetForeground(), "#be123c", "#f08c9b"},
			{"hunk", styles.hunk.GetForeground(), "#0369a1", "#89dceb"},
			{"warning", styles.warning.GetForeground(), "#b45309", "#f5c27c"},
		}
		for _, entry := range colors {
			want := entry.light
			if dark {
				want = entry.dark
			}

			if color.NRGBAModel.Convert(
				entry.color,
			) != color.NRGBAModel.Convert(
				lipgloss.Color(want),
			) {
				t.Errorf("dark=%v: %s changed from %s", dark, entry.name, want)
			}
		}

		if !styles.heading.GetBold() || !styles.selected.GetBold() || !styles.warning.GetBold() {
			t.Fatal("heading, selection, and warning must stay bold")
		}
	}
}

func TestCleanupColorlessViewsMatchV1(t *testing.T) {
	for name, model := range cleanupGoldenModels() {
		t.Run(name, func(t *testing.T) {
			styled := model.View().Content
			for _, profile := range []colorprofile.Profile{
				colorprofile.NoTTY, colorprofile.ASCII, colorprofile.ANSI,
				colorprofile.ANSI256, colorprofile.TrueColor,
			} {
				updated, cmd := model.Update(tea.ColorProfileMsg{Profile: profile})
				model = cleanupTestModel(t, updated)
				view := model.View()

				want := styled
				if profile <= colorprofile.ASCII {
					want = ansi.Strip(styled)
				}

				if cmd != nil || !view.AltScreen || view.Content != want {
					t.Fatalf("%s profile changed the expected styling or layout", profile)
				}
			}

			// Initial profile detection must also suppress styling before the first event.
			model.colorless = true
			if model.View().Content != ansi.Strip(styled) {
				t.Fatal("initial colorless frame contains styling")
			}
		})
	}
}

func TestCleanupColorEnvironmentMatchesV1(t *testing.T) {
	cases := []struct {
		name          string
		noColor       string
		cliColor      string
		cliColorForce string
		disabled      bool
		forced        bool
	}{
		{name: "default"},
		{name: "no color one", noColor: "1", disabled: true},
		{name: "no color yes", noColor: "yes", disabled: true},
		{name: "no color zero", noColor: "0", disabled: true},
		{name: "no color false", noColor: "false", disabled: true},
		{name: "no color arbitrary", noColor: "arbitrary", disabled: true},
		{name: "no color wins force", noColor: "yes", cliColorForce: "1", disabled: true},
		{name: "cli color zero", cliColor: "0", disabled: true},
		{name: "cli color one", cliColor: "1"},
		{name: "cli color false", cliColor: "false"},
		{name: "force zero", cliColor: "0", cliColorForce: "0", disabled: true},
		{name: "force one", cliColor: "0", cliColorForce: "1", forced: true},
		{name: "force yes", cliColor: "0", cliColorForce: "yes", forced: true},
		{name: "force false", cliColor: "0", cliColorForce: "false", forced: true},
		{name: "force arbitrary", cliColorForce: "arbitrary", forced: true},
	}
	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			for _, detected := range []colorprofile.Profile{colorprofile.NoTTY, colorprofile.TrueColor} {
				profile, disabled := tuiColorProfile(
					detected, testcase.noColor, testcase.cliColor, testcase.cliColorForce,
				)

				want := detected
				if testcase.disabled {
					want = colorprofile.ASCII
				} else if testcase.forced {
					want = max(want, colorprofile.ANSI)
				}

				if profile != want || disabled != testcase.disabled {
					t.Fatalf("detected %s: profile=%s, disabled=%v; want %s, %v",
						detected, profile, disabled, want, testcase.disabled)
				}

				model := cleanupModel{
					width: 80, height: 24, noColor: disabled,
					colorless: profile <= colorprofile.ASCII,
				}
				updated, cmd := model.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
				model = cleanupTestModel(t, updated)

				view := model.View().Content
				if cmd != nil || model.colorless != testcase.disabled ||
					(view == ansi.Strip(view)) != testcase.disabled {
					t.Fatal("profile update did not preserve the explicit environment preference")
				}
			}
		})
	}
}

func TestCleanupIgnoresStaleRefreshDuringConfirmation(t *testing.T) {
	for _, phase := range []string{"confirm", "risk", "force"} {
		t.Run(phase, func(t *testing.T) {
			model := cleanupModel{ctx: t.Context()}
			updated, _ := model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
			model = cleanupTestModel(t, updated)
			older := model.generation
			updated, _ = model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
			model = cleanupTestModel(t, updated)
			entry := cleanupEntry{branch: "reviewed"}
			updated, _ = model.Update(
				cleanupLoaded{generation: model.generation, entries: []cleanupEntry{entry}},
			)
			model = cleanupTestModel(t, updated)
			updated, _ = model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})

			model = cleanupTestModel(t, updated)
			if phase != "confirm" {
				updated, _ = model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
				model = cleanupTestModel(t, updated)
			}

			if phase == "force" {
				updated, _ = model.Update(cleanupRiskMsg{risks: []string{"unmerged commits"}})
				model = cleanupTestModel(t, updated)
			}

			for _, entries := range [][]cleanupEntry{nil, {{branch: "different"}}} {
				updated, cmd := model.Update(cleanupLoaded{generation: older, entries: entries})

				actual := cleanupTestModel(t, updated)
				if cmd != nil || len(actual.entries) != 1 || actual.pending != entry ||
					actual.entries[0] != entry || actual.confirm != model.confirm ||
					actual.forceConfirm != model.forceConfirm || actual.busy != model.busy {
					t.Fatal("stale refresh changed the pending action")
				}
			}
		})
	}
}

func TestCleanupDeletionUsesConfirmedSnapshot(t *testing.T) {
	repo := cleanupTestRepo(t)
	cleanupTestGit(t, repo, "branch", "reviewed")
	cleanupTestGit(t, repo, "branch", "other")

	entries, err := cleanupEntriesForRepo(t.Context(), repo)
	if err != nil {
		t.Fatal(err)
	}

	model := cleanupModel{ctx: t.Context(), dir: repo, entries: entries}
	for index, entry := range entries {
		if entry.branch == "reviewed" {
			model.cursor = index
		}
	}

	updated, _ := model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	model = cleanupTestModel(t, updated)
	// Change the displayed selection after confirmation was opened.
	for index, entry := range entries {
		if entry.branch == "other" {
			model.cursor = index
		}
	}

	if model.action() != "delete branch reviewed" {
		t.Fatal("confirmation text changed targets")
	}

	updated, cmd := model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	model = cleanupTestModel(t, updated)
	updated, cmd = model.Update(cmd())
	model = cleanupTestModel(t, updated)
	updated, _ = model.Update(cmd())

	model = cleanupTestModel(t, updated)
	if model.notice != "Removed selected item." {
		t.Fatalf("deletion failed: %s", model.notice)
	}

	if got := cleanupTestGit(t, repo, "branch", "--list", "reviewed"); got != "" {
		t.Fatalf("confirmed branch remains: %s", got)
	}

	if got := cleanupTestGit(t, repo, "branch", "--list", "other"); got == "" {
		t.Fatal("unconfirmed branch was deleted")
	}
}
