package db

type InlandTaxTemplate struct {
	Template string                   `json:"Template"`
	Label    string                   `json:"Label"`
	Currency string                   `json:"Currency"`
	Fields   []InlandTaxTemplateField `json:"Fields"`
}

type InlandTaxTemplateField struct {
	Code      string `json:"Code"`
	Label     string `json:"Label"`
	Currency  string `json:"Currency"`
	SortOrder int    `json:"SortOrder"`
}

type InlandTaxDetail struct {
	Code     string `json:"Code"`
	Label    string `json:"Label"`
	Amount   string `json:"Amount"`
	Currency string `json:"Currency"`
}

var InlandTaxTemplates = map[string]InlandTaxTemplate{
	"DE": {
		Template: "DE",
		Label:    "Deutschland",
		Currency: "EUR",
		Fields: []InlandTaxTemplateField{
			{Code: "capital_gains_tax", Label: "Kapitalertragsteuer", Currency: "EUR", SortOrder: 10},
			{Code: "church_tax", Label: "Kirchensteuer", Currency: "EUR", SortOrder: 20},
			{Code: "solidarity_surcharge", Label: "Solidaritätszuschlag", Currency: "EUR", SortOrder: 30},
		},
	},
}
