package document

type Metadata struct {
	Title         string
	DocumentType  string
	Status        string
	AuthorityRank int
	AccountID     *string
}

var metadata = map[string]Metadata{
	"01_Support_Policy_v3_CURRENT.pdf": {
		Title:         "ParcelPilot Support Policy v3",
		DocumentType:  "support_policy",
		Status:        "CURRENT",
		AuthorityRank: 2,
	},

	"02_Support_Policy_v2_DEPRECATED.pdf": {
		Title:         "ParcelPilot Support Policy v2",
		DocumentType:  "support_policy",
		Status:        "DEPRECATED",
		AuthorityRank: 4,
	},

	"03_Cancellation_and_Service_Credit_SOP_v4.pdf": {
		Title:         "Cancellation & Service Credit SOP v4",
		DocumentType:  "cancellation_sop",
		Status:        "CURRENT",
		AuthorityRank: 2,
	},

	"04_Product_Operations_Guide_and_Known_Issues.pdf": {
		Title:         "Product Operations Guide & Known Issues",
		DocumentType:  "product_documentation",
		Status:        "CURRENT",
		AuthorityRank: 3,
	},

	"05_Northstar_Logistics_Enterprise_Agreement.pdf": {
		Title:         "Northstar Logistics Enterprise Agreement",
		DocumentType:  "customer_agreement",
		Status:        "ACTIVE",
		AuthorityRank: 1,
		AccountID:     stringPtr("ACCT-001"),
	},

	"06_LumenWorks_Service_Agreement.pdf": {
		Title:         "LumenWorks Service Agreement",
		DocumentType:  "customer_agreement",
		Status:        "ACTIVE",
		AuthorityRank: 1,
		AccountID:     stringPtr("ACCT-002"),
	},
}

func MetadataFor(filename string) (Metadata, bool) {
	value, ok := metadata[filename]
	return value, ok
}

func stringPtr(value string) *string {
	return &value
}
