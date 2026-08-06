# Providers

Each provider is a small adapter behind one interface (`integrations.Source`)
that pulls that vendor's own cost/usage API and emits FOCUS 1.2 records. This
directory documents, per provider, the credentials and configuration needed to
run an export.

## Available

| Provider | Domain | Auth | Doc |
| --- | --- | --- | --- |
| PlanetScale | Managed database (infra cost) | Service token | [planetscale.md](planetscale.md) |

## Planned

| Provider | Domain | Notes |
| --- | --- | --- |
| Anthropic | AI model cost | Token-level real cost; token detail as `x_` extensions |
| OpenAI | AI model cost | Model-level tokens + coarse cost |

## Documenting a new provider

Copy the structure of [planetscale.md](planetscale.md):

1. **Overview** - what the provider bills for and the FOCUS grain (one record per what).
2. **Environment variables** - a table: name, required, description.
3. **Credential setup** - step-by-step to obtain the credential, and the minimum
   scopes/permissions it needs.
4. **API endpoints used** - the exact endpoints the adapter calls, so operators
   can reason about access and rate limits.
5. **FOCUS mapping** - a table from FOCUS column to provider field, including any
   `x_` extension columns.
6. **Example** - a runnable command and a representative output record.
7. **Notes and limitations** - granularity, currency, known gaps.
