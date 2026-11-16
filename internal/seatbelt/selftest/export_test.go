package selftest

// Every package with tests must import this package in its
// selftest_test.go (TestEveryPackageWithTestsImportsTheSelfTest). Here,
// that forces selftest_test.go into package selftest_test, which cannot
// access the unexported tests. Tests exports it for that file; being in a
// _test.go file, the export exists only in the test binary.
var Tests = tests
