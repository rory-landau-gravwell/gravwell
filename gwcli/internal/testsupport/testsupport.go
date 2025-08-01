/*************************************************************************
 * Copyright 2025 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

// Package testsupport provides utility functions useful across disparate testing packages
//
// TT* functions are for use with tests that rely on TeaTest.
// Friendly reminder: calling tm.Type() with "\n"/"\t"/etc does not, at the time of writing, actually trigger the corresponding key message.
package testsupport

import (
	"fmt"
	"io"
	"maps"
	"reflect"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

const (
	// This adds a short pause after TTSendSpecial sends.
	// This is because tea.Cmds are async and time-unbounded.
	// In other words, we need to buy Bubbletea extra time for the messages to propagate down to the final action model.
	// If we don't, MatchGolden can fail even if the final-final output is correct.
	SendSpecialPause time.Duration = 50 * time.Millisecond
)

//#region TeaTest

// A MessageRcvr is anything that can accept tea.Msgs via a .Send() method.
type MessageRcvr interface {
	Send(tea.Msg)
}

// TTSendSpecial submits a KeyMsg containing the special key (CTRL+C, ESC, etc) to the test model.
// Ensures the KeyMsg is well-formatted, as ill-formatted KeyMsgs are silently dropped (as they are not read as KeyMsgs) or cause panics.
//
// For use with TeaTests.
func TTSendSpecial(r MessageRcvr, kt tea.KeyType) {
	r.Send(tea.KeyMsg(tea.Key{Type: kt, Runes: []rune{rune(kt)}}))
	time.Sleep(SendSpecialPause)

}

// Type adds teatest.TestModel.Type() to a normal tea.Program.
func Type(prog *tea.Program, text string) {
	for _, r := range text {
		prog.Send(tea.KeyMsg(
			tea.Key{Type: tea.KeyRunes, Runes: []rune{rune(r)}}))
	}
}

// TTMatchGolden compares the output (final View) of tm against the test's associated output file.
//
// ! This blocks until tm returns.
func TTMatchGolden(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	if err != nil {
		t.Error(err)
	}
	// matches on the golden file with the test function's name
	teatest.RequireEqualOutput(t, out)
}

//#endregion TeaTest

// ExpectedActual returns a string declaring what was expected and what we got instead.
// ! Prefixes the string with a newline.
func ExpectedActual(expected, actual any) string {
	return fmt.Sprintf("\n\tExpected:'%+v'\n\tGot:'%+v'", expected, actual)
}

// NonZeroExit calls Fatal if code is <> 0.
func NonZeroExit(t *testing.T, code int, stderr string) {
	t.Helper()
	if code != 0 {
		t.Fatalf("non-zero exit code %v.\nstderr: '%v'", code, stderr)
	}
}

// SlicesUnorderedEqual compares the elements of two slices for equality (and equal count) without caring about the order of the elements.
// Copied from my (rflandau) Orv test code.
func SlicesUnorderedEqual(a []string, b []string) bool {
	// convert each slice into map of key --> count
	am := make(map[string]uint)
	for _, k := range a {
		am[k] += 1
	}
	bm := make(map[string]uint)
	for _, k := range b {
		bm[k] += 1
	}

	return maps.Equal(am, bm)
}

// ExtractPrintLineMessageString attempts to pull the messageBody string out from the tea.printLineMessage private struct by reflecting into it.
// It can parse sequences/batches.
// Returns the string on success; fatal on failure.
// Only operates at the first layer; will not traverse nested sequence/batches
//
// If !sliceOK, then it will fail if the given command returned a tea.Batch or tea.Sequence.
// sequenceIndex sets the expected index of the printLineMessage if the cmd is a tea.Batch or tea.Sequence.
// Has no effect if !sliceOK.
func ExtractPrintLineMessageString(t *testing.T, cmd tea.Cmd, sliceOK bool, sequenceIndex uint) string {
	t.Helper()
	voMsg := reflect.ValueOf(cmd())
	t.Logf("Update msg kind: %v", voMsg.Kind())
	// this will be a slice if it is a sequence or a struct if a single msg
	var voPLM reflect.Value
	if voMsg.Kind() == reflect.Slice {
		if !sliceOK {
			t.Fatal("message is a slice; slices were marked unacceptable")
		}
		// ensure the sequence/batch is at least as large as the index
		if voMsg.Len() <= int(sequenceIndex) {
			t.Fatal("sequence/batch is too short.", ExpectedActual(fmt.Sprintf("at least %v", sequenceIndex), voMsg.Len()))
		}
		// select a single item
		voInnerCmd := voMsg.Index(int(sequenceIndex))
		// voItm1 should now be a Cmd that returns a printLineMessage
		if voInnerCmd.Kind() != reflect.Func {
			t.Fatal(ExpectedActual(reflect.Func, voMsg.Kind()))
		}
		// invoke, check that exactly 1 value (the message) is returned
		if voInnerMsg := voInnerCmd.Call(nil); len(voInnerMsg) != 1 {
			t.Fatal("bad output count", ExpectedActual(1, len(voInnerMsg)))
		} else {
			voPLM = voInnerMsg[sequenceIndex]
		}
	} else { // not a sequence, just a raw printLineMessage (or an interface of a Msg)
		voPLM = voMsg
	}

	// if the Message is still in interface form, we need to dereference it
	if voPLM.Kind() == reflect.Interface {
		voPLM = voPLM.Elem()
	}
	if voPLM.Kind() != reflect.Struct {
		t.Fatal(ExpectedActual(reflect.Struct, voPLM.Kind()))
	}

	voMessageBody := voPLM.FieldByName("messageBody")
	if voMessageBody.Kind() != reflect.String {
		t.Fatal(ExpectedActual(reflect.String, voMessageBody.Kind()))
	}
	return voMessageBody.String()
}
