// Package ui asks the user questions: confirmations, setup approvals and
// pickers (ADR 0027).
package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"charm.land/huh/v2"
	"charm.land/huh/v2/spinner"
	"golang.org/x/term"
)

// ErrDeclined is returned when the user answers no.
var ErrDeclined = errors.New("declined")

// Prompter asks yes/no questions and lets the user pick.
type Prompter interface {
	// Interactive reports whether crannog may ask questions: stdin and
	// stdout are terminals.
	Interactive() bool
	Ask(question string) (bool, error)
	Pick(title string, options []string) (string, error)
	// Wait shows title while action runs. Ctrl-C cancels the action's
	// context and is an error.
	Wait(title string, action func(context.Context) error) error
}

// UI asks questions and prints messages.
type UI struct {
	Prompter
	// Out receives crannog's messages; stdout stays the session's.
	Out io.Writer
	// Yes answers confirmations with yes (--yes).
	Yes bool
}

// Printf prints a message.
func (u *UI) Printf(format string, args ...any) {
	fmt.Fprintf(u.Out, format, args...)
}

// AskConfirmation asks a confirmation. --yes answers it; without a
// terminal and without --yes it is an error.
func (u *UI) AskConfirmation(question string) error {
	if u.Yes {
		return nil
	}
	if !u.Interactive() {
		return fmt.Errorf("confirmation needed (%s); pass --yes or run in a terminal", question)
	}
	ok, err := u.Ask(question)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDeclined
	}
	return nil
}

// AskSetupApproval asks for a setup approval. --yes never answers it;
// without a terminal it is an error.
func (u *UI) AskSetupApproval(question string) error {
	if !u.Interactive() {
		return errors.New("the setup needs your approval; run crannog in a terminal (--yes does not approve setups)")
	}
	ok, err := u.Ask(question)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDeclined
	}
	return nil
}

// PickOne lets the user pick one of options; without a terminal it is an
// error naming what is missing.
func (u *UI) PickOne(title, missing string, options []string) (string, error) {
	if !u.Interactive() {
		return "", fmt.Errorf("%s is missing (no terminal to pick one)", missing)
	}
	return u.Pick(title, options)
}

// WaitFor shows title while action runs, with a spinner in a terminal.
// There, Ctrl-C cancels the action's context and is an error.
func (u *UI) WaitFor(title string, action func(context.Context) error) error {
	if !u.Interactive() {
		u.Printf("%s\n", title)
		return action(context.Background())
	}
	return u.Wait(title, action)
}

// Terminal asks questions on the terminal with huh.
type Terminal struct{}

func (Terminal) Interactive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func (Terminal) Ask(question string) (bool, error) {
	var ok bool
	err := huh.NewConfirm().Title(question).Affirmative("Yes").Negative("No").Value(&ok).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return false, ErrDeclined
	}
	return ok, err
}

func (Terminal) Pick(title string, options []string) (string, error) {
	var v string
	err := huh.NewSelect[string]().Title(title).Options(huh.NewOptions(options...)...).Value(&v).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return "", ErrDeclined
	}
	return v, err
}

func (Terminal) Wait(title string, action func(context.Context) error) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var actionErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		actionErr = action(ctx)
	}()
	err := spinner.New().Title(" " + title).ActionWithErr(func(context.Context) error {
		<-done
		return nil
	}).Run()
	select {
	case <-done:
		return actionErr
	default:
	}
	// The spinner ended first: the action stops before Wait returns.
	cancel()
	<-done
	return err
}
