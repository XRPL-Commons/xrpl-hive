package main

import (
	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func credentialCreateAcceptDelete() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "credential_create_accept_delete",
		Description: "Full credential lifecycle: create, accept, delete.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			subject := accounts[1]

			// CredentialCreate: issuer creates credential for subject.
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "CredentialCreate",
				"Subject":         subject.Address,
				"CredentialType":  "4B5943", // "KYC" in hex
			})
			if err != nil {
				t.Fatal("credential create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// CredentialAccept: subject accepts.
			acceptResult, err := rpc.Submit(subject.Secret, subject.Address, map[string]interface{}{
				"TransactionType": "CredentialAccept",
				"Issuer":          issuer.Address,
				"CredentialType":  "4B5943",
			})
			if err != nil {
				t.Fatal("credential accept:", err)
			}
			assertEngineResult(t, acceptResult, "tesSUCCESS")
			waitSettled(rpc)

			// CredentialDelete: subject can delete their credential.
			delResult, err := rpc.Submit(subject.Secret, subject.Address, map[string]interface{}{
				"TransactionType": "CredentialDelete",
				"Issuer":          issuer.Address,
				"Subject":         subject.Address,
				"CredentialType":  "4B5943",
			})
			if err != nil {
				t.Fatal("credential delete:", err)
			}
			assertEngineResult(t, delResult, "tesSUCCESS")
			t.Log("credential lifecycle complete: create -> accept -> delete")
		},
	}
}
