# Yuice Protocol

Go infrastructure for programmable finance workflows around tokenized
real-world assets. The initial Yuice-specific code models payment proposals
through the stages described on the [Yuice Labs website](https://www.yuicelabs.com/):
connect asset activity, reconcile financial inputs, apply governance rules,
then authorize payment execution.

## Current scope

The [`yuice/finance`](./yuice/finance) package provides a domain model for
asset-level payment proposals. It uses integer token base units, explicit
chain/token/recipient allowlists, payment limits, and distinct human approvals.
It does not include persistence, private-key handling, RPC submission, or a
token transfer adapter. It cannot move funds.

## Blockchain client

This repository also contains the Go blockchain client and node tools inherited
from the upstream project. Building the node binaries requires Go and a C
compiler. Run `make all` to build the available commands; the repository uses
Git submodules, and `make submodules` initializes the system-contracts
submodule needed for code generation.


This is an early development base. Do not use it to custody assets or execute
production payments. A production integration requires a reviewed network and
token configuration, custody model, durable audit and idempotency controls,
recovery behavior, and approval policy.
