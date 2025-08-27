
# Security and quality checks
security-scan:
  name: Security Scan
  runs-on: ubuntu-latest
  steps:
    - name: Harden Runner
      uses: step-security/harden-runner@v2
      with:
        egress-policy: audit

    - name: Checkout
      uses: actions/checkout@v4

    - name: Setup Go
      uses: actions/setup-go@v5
      with:
        go-version: ${{ env.GO_VERSION }}
        check-latest: true

    # Run gosec directly (no dead action wrapper)
    - name: Install and Run Gosec
      run: |
        go install github.com/securego/gosec/v2/cmd/gosec@latest
        $(go env GOPATH)/bin/gosec -fmt sarif -out gosec.sarif ./...

    - name: Upload SARIF file
      uses: github/codeql-action/upload-sarif@v3
      with:
        sarif_file: gosec.sarif

    - name: Run govulncheck
      run: |
        go install golang.org/x/vuln/cmd/govulncheck@latest
        $(go env GOPATH)/bin/govulncheck ./...
