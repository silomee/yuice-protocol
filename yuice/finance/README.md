# Yuice finance workflow

This Go package implements the first three stages shown in Yuice's public
operating model: connect a payment proposal to an asset, mark its financial
inputs reconciled, and apply explicit governance rules before authorization.
The site describes execution as a later step coordinated through compatible
payment infrastructure.

The package represents values as integer token base units, checks chain,
token and recipient allowlists, enforces a per-payment maximum, and requires
distinct human approvals. A caller must provide every policy setting; there
are no permissive defaults.

This is a domain layer only. It has no database, transaction signer, RPC
client, or token-transfer implementation. Do not use it to move funds until
the network, token, custody model, persistence and recovery behavior, and
approval policy have been reviewed and implemented in separate adapters.
