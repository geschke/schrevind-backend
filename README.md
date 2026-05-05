# Schrevind

**Schrevind** is a small, self-hosted application for tracking dividend income and similar capital distributions from stocks and ETFs.

The project was created to replace a personal Excel spreadsheet that had grown over time and contained various VBA helpers for calculations and reporting. Instead of relying on proprietary spreadsheet software, Schrevind provides a simple and transparent system with a structured data model and a lightweight web interface.

The focus of the application is deliberately narrow:
Schrevind records **actual dividend payments and related tax information** based on brokerage statements. It does **not** attempt to be a full portfolio management system. There is no market data integration, no price tracking, and no forecasting. The goal is simply to document what has actually been paid and to generate useful summaries.

Typical use cases include:

* Recording dividend payments from stocks and ETF distributions
* Tracking withholding taxes and related tax details
* Handling different currencies as reported by brokerage statements
* Comparing dividend income across months and years
* Generating simple reports based on recorded data

Schrevind is designed as a **self-hosted web application**. The backend is written in Go and uses SQLite for storage. The frontend will be implemented as a lightweight Vue-based interface.

The project is intentionally kept small, transparent, and easy to run locally or on a small server (for example a home server or a Raspberry Pi).

More documentation will be added as the project evolves.

## Configuration Notes

When the web UI is enabled, Schrevind requires a TOTP encryption key for
two-factor authentication secrets:

```yaml
web_ui:
  totp_encryption_key: "<hex-encoded 32-byte key>"
```

Generate a key with:

```bash
openssl rand -hex 32
```
