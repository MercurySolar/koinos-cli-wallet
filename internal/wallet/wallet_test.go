package wallet

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/shopspring/decimal"
)

func TestSatoshiToDecimal(t *testing.T) {
	v, err := SatoshiToDecimal(100000000, 8)
	if err != nil {
		t.Error(err)
	}

	if !v.Equal(decimal.NewFromFloat(1.0)) {
		t.Error("Expected 1.0, got", v)
	}

	v, err = SatoshiToDecimal(1000, 1)
	if err != nil {
		t.Error(err)
	}

	if !v.Equal(decimal.NewFromFloat(100.0)) {
		t.Error("Expected 100.0, got", v)
	}

	v, err = SatoshiToDecimal(12345678, 3)
	if err != nil {
		t.Error(err)
	}

	if !v.Equal(decimal.NewFromFloat(12345.678)) {
		t.Error("Expected 1234.5678, got", v)
	}
}

func makeTestParser() *CommandParser {
	// Construct the command parser
	cs := NewCommandSet()

	cs.AddCommand(NewCommandDeclaration("test_address", "Test command which takes an address", false, nil, *NewCommandArg("address", Address)))
	cs.AddCommand(NewCommandDeclaration("test_string", "Test command which takes a string", false, nil, *NewCommandArg("string", String)))
	cs.AddCommand(NewCommandDeclaration("test_none", "Test command which takes no arguments", false, nil))
	cs.AddCommand(NewCommandDeclaration("test_none2", "Another test command which takes no arguments", false, nil))
	cs.AddCommand(NewCommandDeclaration("test_multi", "Test command which takes multiple arguments, and of different types", false, NewGenerateKeyCommand,
		*NewCommandArg("arg0", Address), *NewCommandArg("arg1", String), *NewCommandArg("arg2", Amount), *NewCommandArg("arg0", String)))

	parser := NewCommandParser(cs)

	return parser
}

func TestBasicParser(t *testing.T) {
	parser := makeTestParser()

	// Test parsing several commands
	results, err := parser.Parse("test_address 1iwBq2QAax2URVqU2h878hTs8DFFKADMk; test_none; test_none2")
	if err != nil {
		t.Error(err)
	}

	if results.Len() != 3 {
		t.Error("Expected 3 result, got", results.Len())
	}

	results, err = parser.Parse("asdasd")
	if err == nil {
		t.Error("Expected error, got none")
	}

	if !errors.Is(err, ErrUnknownCommand) {
		t.Error("Expected error", ErrUnknownCommand, ", got", err)
	}

	if results.CommandResults[0].CurrentArg != -1 {
		t.Error("Expected current arg to be -1, got", results.CommandResults[0].CurrentArg)
	}

	results, err = parser.Parse("asdasd ")
	if err == nil {
		t.Error("Expected error, got none")
	}

	if !errors.Is(err, ErrUnknownCommand) {
		t.Error("Expected error", ErrUnknownCommand, ", got", err)
	}

	if results.CommandResults[0].CurrentArg != 0 {
		t.Error("Expected current arg to be 0, got", results.CommandResults[0].CurrentArg)
	}

	// Test parsing empty inputs
	results, err = parser.Parse("")
	if err != nil {
		t.Error(err)
	}

	if results.Len() != 0 {
		t.Error("Expected 0 results, got", results.Len())
	}

	results, err = parser.Parse("    ")
	if err != nil {
		t.Error(err)
	}

	if results.Len() != 0 {
		t.Error("Expected 0 results, got", results.Len())
	}

	// Test nonsensical string of empty commands
	results, err = parser.Parse(" ; ;; ;; ;;;;    ;     ;  ;    ")
	if err == nil {
		t.Error("Expected error, got none")
	}

	if results.Len() != 0 {
		t.Error("Expected 0 results, got", results.Len())
	}

	// Test valid command followed by empty commands
	results, err = parser.Parse("test_address 1iwBq2QAax2URVqU2h878hTs8DFFKADMk;  ;;  ; ;; test_none")
	if err == nil {
		t.Error("Expected error, got none")
	}

	if !errors.Is(err, ErrEmptyCommandName) {
		t.Error("Expected error", ErrEmptyCommandName, ", got", err)
	}

	if results.Len() != 1 {
		t.Error("Expected 1 result, got", results.Len())
	}
}

// Test that parser correctly parses terminators
func TestParserTermination(t *testing.T) {
	parser := makeTestParser()

	checkTerminators(t, parser, "test_address 1iwBq2QAax2URVqU2h878hTs8DFFKADMk", []TerminationStatus{Input})
	checkTerminators(t, parser, "test_address 1iwBq2QAax2URVqU2h878hTs8DFFKADMk;", []TerminationStatus{Command})
	checkTerminators(t, parser, "  test_address 1iwBq2QAax2URVqU2h878hTs8DFFKADMk   ", []TerminationStatus{Input})
	checkTerminators(t, parser, "      test_address 1iwBq2QAax2URVqU2h878hTs8DFFKADMk  ;   ", []TerminationStatus{Command})
	checkTerminators(t, parser, "test_address 1iwBq2QAax2URVqU2h878hTs8DFFKADMk test_address 1iwBq2QAax2URVqU2h878hTs8DFFKADMk", []TerminationStatus{None})
	checkTerminators(t, parser, "test_address 1iwBq2QAax2URVqU2h878hTs8DFFKADMk; test_address 1iwBq2QAax2URVqU2h878hTs8DFFKADMk", []TerminationStatus{Command, Input})
	checkTerminators(t, parser, "test_address 1iwBq2QAax2URVqU2h878hTs8DFFKADMk; test_address 1iwBq2QAax2URVqU2h878hTs8DFFKADMk;", []TerminationStatus{Command, Command})
}

func checkTerminators(t *testing.T, parser *CommandParser, input string, terminators []TerminationStatus) {
	results, err := parser.Parse(input)
	if err != nil {
		t.Error(err)
	}

	if results.Len() != len(terminators) {
		t.Error("Expected", len(terminators), "results, got", results.Len())
	}

	for i, result := range results.CommandResults {
		if result.Termination != terminators[i] {
			t.Error("Expected terminator", terminators[i], "got", result.Termination)
		}
	}
}

func TestWalletFile(t *testing.T) {
	testKey := []byte{0x03, 0x02, 0x01, 0x0A, 0x0B, 0x0C}

	// Storage of test bytes
	file, err := ioutil.TempFile("", "wallet_test_*")
	defer os.Remove(file.Name())

	if err != nil {
		t.Error(err.Error())
	}

	err = CreateWalletFile(file, "my_password", testKey)

	if err != nil {
		t.Error(err.Error())
	}

	file.Close()

	// A successful retrieval of stored bytes
	file, err = os.OpenFile(file.Name(), os.O_RDONLY, 0600)

	if err != nil {
		t.Error(err.Error())
	}

	result, err := ReadWalletFile(file, "my_password")

	if err != nil {
		t.Error(err.Error())
	}

	if !bytes.Equal(testKey, result) {
		t.Error("retrieved private key from wallet file mismatch")
	}

	file.Close()

	// An usuccessful retrieval of stored bytes using wrong password
	file, err = os.OpenFile(file.Name(), os.O_RDONLY, 0600)

	if err != nil {
		t.Error(err.Error())
	}

	_, err = ReadWalletFile(file, "not_my_password")

	if err == nil {
		t.Error("retrieved private key without correct passphrase")
	}

	file.Close()

	// Prevent an empty passphrase
	errfile, err := ioutil.TempFile("", "wallet_test_*")
	defer os.Remove(errfile.Name())

	if err != nil {
		t.Error(err.Error())
	}

	err = CreateWalletFile(errfile, "", testKey)

	if err != ErrEmptyPassphrase {
		t.Error("an empty passphrase is not allowed")
	}

	errfile.Close()
}

func TestParseMetrics(t *testing.T) {
	// Construct the command parser
	parser := makeTestParser()

	// Test parsing a half finished command
	checkMetrics("test_mu", parser, t, true, 0, -1)

	// Test parsing a finished command
	checkMetrics("test_multi", parser, t, true, 0, -1)

	// Test parsing a finished command with a space
	checkMetrics("test_multi ", parser, t, true, 0, 0)

	// Test parsing the rest of the arguments address string amount string
	checkMetrics("test_multi 1iwBq2QA", parser, t, true, 0, 0)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk", parser, t, true, 0, 0)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk ", parser, t, true, 0, 1)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword str", parser, t, true, 0, 1)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string'", parser, t, true, 0, 1)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string' ", parser, t, true, 0, 2)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string' 50.403", parser, t, true, 0, 2)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string' 50.403873", parser, t, true, 0, 2)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string' 50.403873 ", parser, t, true, 0, 3)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string' 50.403873 basic_str", parser, t, false, 0, 3)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string' 50.403873 basic_str ", parser, t, false, 0, 3) // What should happen here?
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string' 50.403873 basic_str;", parser, t, false, 1, -1)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string' 50.403873 basic_str; ", parser, t, false, 1, -1)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string' 50.403873 basic_str; tes", parser, t, true, 1, -1)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string' 50.403873 basic_str; test_string", parser, t, true, 1, -1)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string' 50.403873 basic_str; test_string ", parser, t, true, 1, 0)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string' 50.403873 basic_str; test_string \"abc", parser, t, true, 1, 0)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string' 50.403873 basic_str; test_string \"ab' \\\"cdef\"", parser, t, false, 1, 0)
	checkMetrics("test_multi 1iwBq2QAax2URVqU2h878hTs8DFFKADMk 'a multiword string' 50.403873 basic_str; test_string \"ab' \\\"cdef\";", parser, t, false, 2, -1)

}

func checkMetrics(input string, parser *CommandParser, t *testing.T, expectError bool, index int, arg int) {
	res, err := parser.Parse(input)
	if err == nil && expectError {
		t.Error("Expected error, got none")
	} else if err != nil && !expectError {
		t.Error(err.Error())
	}

	metrics := res.Metrics()

	if metrics.CurrentResultIndex != index {
		t.Error("Expected current result index to be", index, "got", metrics.CurrentResultIndex)
	}

	if metrics.CurrentArg != arg {
		t.Error("Expected current arg to be", arg, "got", metrics.CurrentArg)
	}
}
