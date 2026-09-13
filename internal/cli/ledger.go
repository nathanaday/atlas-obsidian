package cli

import "github.com/nathanaday/claude-atlas/internal/ledger"

func ledgerUpdate(id string, ingested bool, pages []string, authority, title, notes string) ledger.Update {
	return ledger.Update{ID: id, Ingested: ingested, Pages: pages, Authority: authority, Title: title, Notes: notes}
}
