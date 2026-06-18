# Security Policy

## Supported Versions

This project is under active development. Only the latest version (latest commit on `main`) is currently supported with security updates.

| Version | Supported |
|---------|-----------|
| main (latest) | ✅ |
| Previous releases | ❌ |

## Reporting a Vulnerability

To report a security vulnerability, please use **GitHub Security Advisories**:

1. Go to https://github.com/nbyl/metio/security/advisories
2. Click **"New draft security advisory"**
3. Fill in the details of the vulnerability

Alternatively, you can report via email to [nico@nicolas-byl.eu](mailto:nico@nicolas-byl.eu).

### What to Include

- Type of vulnerability
- Steps to reproduce
- Affected versions
- Any potential mitigations you've identified

### Response Timeline

- **Acknowledgement**: within 48 hours
- **Initial assessment**: within 5 business days
- **Fix timeline**: communicated after assessment

### Disclosure Policy

We follow a coordinated disclosure process:

1. Reporter submits vulnerability (private)
2. We acknowledge and assess
3. We develop and test a fix
4. Fix is deployed
5. Vulnerability is publicly disclosed after a reasonable period

## Security Best Practices

### Environment Variables

This project relies heavily on environment variables for sensitive configuration. Never commit `.env` files or expose secrets in logs, error messages, or issue reports.

### Authentication

- OAuth 2.0 is used for user authentication
- Access is restricted via `ALLOWED_USERS` configuration
- Session keys should be strong, unique, and rotated regularly

### Infrastructure

- All traffic is served over HTTPS
- GCP IAM roles follow least-privilege principle
- Container images are built from trusted base images
- Dependencies are managed via Go modules and npm with lockfiles
