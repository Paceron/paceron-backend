.PHONY: test coverage coverage-html

test:
	go test ./... -v

coverage:
	mkdir -p ci/test_coverage
	go list -f '{{if .TestGoFiles}}{{.ImportPath}}{{end}}' ./... | xargs go test -coverpkg=./... -coverprofile=ci/test_coverage/coverage.out -covermode=atomic
	go tool cover -func=ci/test_coverage/coverage.out

coverage-html: coverage
	go tool cover -html=ci/test_coverage/coverage.out -o ci/test_coverage/coverage.html
