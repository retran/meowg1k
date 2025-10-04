# Security Policy

The `meowg1k` team takes security seriously. We appreciate the efforts of security researchers and the community to help us maintain a high standard of security.

---

## Philosophies

Our security model is guided by our core principles:

- **Local-First:** The tool is designed to work completely offline with local models, ensuring your code never has to leave your machine unless you explicitly configure a cloud provider.
- **Security by Design:** We follow a "zero trust" model. The application never stores or persists user secrets like API keys. They are only held in memory for the duration of a request.
- **Transparency:** Our entire software supply chain is open source. There are no proprietary black boxes.

---

## Supported Versions

Only the latest major version of `meowg1k` receives security updates. Please ensure you are running the most recent release.

---

## Vulnerability Reporting

Please **do not** report security vulnerabilities through public GitHub issues.

To report a vulnerability, please use GitHub's private vulnerability reporting feature. This ensures that the information is disclosed responsibly.

1. Go to the [**"Security" tab of the `meowg1k` repository**](https://github.com/retran/meowg1k/security).
2. Click on **"Report a vulnerability"**.
3. Fill out the form with as much detail as possible, including:
   - A clear description of the vulnerability.
   - The steps required to reproduce it.
   - The potential impact of the vulnerability.
   - Any known workarounds.

You can expect a response from us within **48 hours** to acknowledge receipt of your report. We will work with you to understand the issue and coordinate a fix and disclosure.

---

## Release Verification

All official release artifacts are signed with [Sigstore cosign](https://docs.sigstore.dev/cosign/overview) and include a Software Bill of Materials (SBOM).

You can verify the integrity of a release binary using the following command:

```bash
cosign verify-blob \
  --certificate-identity-regexp '[https://github.com/retran/meowg1k](https://github.com/retran/meowg1k)' \
  --certificate-oidc-issuer '[https://token.actions.githubusercontent.com](https://token.actions.githubusercontent.com)' \
  --signature meowg1k_<version>_linux_amd64.tar.gz.sig \
  meowg1k_<version>_linux_amd64.tar.gz
```
