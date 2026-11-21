package auditor

import (
	"regexp"
	"strings"
)

type gaapPattern struct {
	tag     string
	regex   *regexp.Regexp
	section string // optional section filter
}

var gaapRules = []gaapPattern{
	// Income Statement
	{
		tag:   "us-gaap:Revenues",
		regex: regexp.MustCompile(`(?i)\b(total\s+(?:net\s+)?sales|total\s+revenue|net\s+sales|operating\s+revenues?)\b`),
	},
	{
		tag:   "us-gaap:CostOfGoodsAndServicesSold",
		regex: regexp.MustCompile(`(?i)\b(total\s+cost\s+of\s+sales|cost\s+of\s+(?:sales|goods\s+sold|revenue|products|services))\b`),
	},
	{
		tag:   "us-gaap:GrossProfit",
		regex: regexp.MustCompile(`(?i)\b(gross\s+profit|gross\s+margin)\b`),
	},
	{
		tag:   "us-gaap:ResearchAndDevelopmentExpense",
		regex: regexp.MustCompile(`(?i)\b(research\s+and\s+development|r&d)\b`),
	},
	{
		tag:   "us-gaap:SellingGeneralAndAdministrativeExpense",
		regex: regexp.MustCompile(`(?i)\b(selling,\s*general\s+and\s+administrative|sg&a|selling\s+and\s+marketing)\b`),
	},
	{
		tag:   "us-gaap:OperatingExpenses",
		regex: regexp.MustCompile(`(?i)\b(total\s+operating\s+expenses|operating\s+expenses)\b`),
	},
	{
		tag:   "us-gaap:OperatingIncomeLoss",
		regex: regexp.MustCompile(`(?i)\b(operating\s+income|operating\s+loss|operating\s+profit|income\s+from\s+operations)\b`),
	},
	{
		tag:   "us-gaap:NonoperatingIncomeExpense",
		regex: regexp.MustCompile(`(?i)\b(other\s+income,\s*net|total\s+other\s+income|non-operating\s+income|interest\s+income)\b`),
	},
	{
		tag:   "us-gaap:IncomeLossFromContinuingOperationsBeforeIncomeTaxesMinorityInterestAndIncomeLossFromEquityMethodInvestments",
		regex: regexp.MustCompile(`(?i)\b(income\s+before\s+(?:provision\s+for\s+)?(?:income\s+)?taxes|earnings\s+before\s+taxes)\b`),
	},
	{
		tag:   "us-gaap:IncomeTaxExpenseBenefit",
		regex: regexp.MustCompile(`(?i)\b(provision\s+for\s+(?:income\s+)?taxes|income\s+tax\s+expense)\b`),
	},
	{
		tag:   "us-gaap:NetIncomeLoss",
		regex: regexp.MustCompile(`(?i)\b(net\s+income|net\s+loss|net\s+earnings)\b`),
	},

	// Balance Sheet - Assets
	{
		tag:   "us-gaap:CashAndCashEquivalentsAtCarryingValue",
		regex: regexp.MustCompile(`(?i)\b(cash\s+and\s+cash\s+equivalents|cash\s+and\s+equivalents)\b`),
	},
	{
		tag:   "us-gaap:MarketableSecuritiesCurrent",
		regex: regexp.MustCompile(`(?i)\b(marketable\s+securities|short-term\s+investments)\b`),
	},
	{
		tag:   "us-gaap:AccountsReceivableNetCurrent",
		regex: regexp.MustCompile(`(?i)\b(accounts\s+receivable|trade\s+receivables)\b`),
	},
	{
		tag:   "us-gaap:InventoryNet",
		regex: regexp.MustCompile(`(?i)\b(inventories|merchandise\s+inventories)\b`),
	},
	{
		tag:   "us-gaap:OtherAssetsNoncurrent",
		regex: regexp.MustCompile(`(?i)\b(other\s+(?:non-current|noncurrent)\s+assets|other\s+assets|non-current\s+assets|noncurrent\s+assets)\b`),
	},
	{
		tag:   "us-gaap:AssetsCurrent",
		regex: regexp.MustCompile(`(?i)(?:^|[^a-zA-Z0-9-])(total\s+current\s+assets|current\s+assets)\b`),
	},
	{
		tag:   "us-gaap:PropertyPlantAndEquipmentNet",
		regex: regexp.MustCompile(`(?i)\b(property,\s*plant\s+and\s+equipment|pp&e)\b`),
	},
	{
		tag:   "us-gaap:Assets",
		regex: regexp.MustCompile(`(?i)\b(total\s+assets)\b`),
	},

	// Balance Sheet - Liabilities & Equity
	{
		tag:   "us-gaap:AccountsPayableCurrent",
		regex: regexp.MustCompile(`(?i)\b(accounts\s+payable|trade\s+payables)\b`),
	},
	{
		tag:   "us-gaap:LiabilitiesCurrent",
		regex: regexp.MustCompile(`(?i)(?:^|[^a-zA-Z0-9-])(total\s+current\s+liabilities|current\s+liabilities)\b`),
	},
	{
		tag:   "us-gaap:LongTermDebtNoncurrent",
		regex: regexp.MustCompile(`(?i)\b(long-term\s+debt|term\s+debt)\b`),
	},
	{
		tag:   "us-gaap:LiabilitiesAndStockholdersEquity",
		regex: regexp.MustCompile(`(?i)\b(total\s+liabilities\s+and\s+(?:(?:shareholders|stockholders)['’]?\s+)?equity)\b`),
	},
	{
		tag:   "us-gaap:StockholdersEquity",
		regex: regexp.MustCompile(`(?i)\b(total\s+(?:shareholders'|stockholders')\s+equity|shareholders'\s+equity|total\s+equity)\b`),
	},
	{
		tag:   "us-gaap:Liabilities",
		regex: regexp.MustCompile(`(?i)\b(total\s+liabilities)\b`),
	},
	{
		tag:   "us-gaap:CommonStocksIncludingAdditionalPaidInCapital",
		regex: regexp.MustCompile(`(?i)\b(common\s+stock|additional\s+paid-in\s+capital)\b`),
	},
	{
		tag:   "us-gaap:RetainedEarningsAccumulatedDeficit",
		regex: regexp.MustCompile(`(?i)\b(retained\s+earnings|accumulated\s+deficit)\b`),
	},
}

// MapLabelToGAAP resolves an extracted financial label to its standardized US-GAAP concept tag.
// Returns an empty string if no unambiguous match is found.
func MapLabelToGAAP(label, section string) string {
	cleaned := strings.TrimSpace(label)
	if cleaned == "" {
		return ""
	}

	for _, rule := range gaapRules {
		if rule.section != "" && !strings.Contains(strings.ToLower(section), strings.ToLower(rule.section)) {
			continue
		}
		if rule.regex.MatchString(cleaned) {
			return rule.tag
		}
	}

	return ""
}
