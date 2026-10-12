package domain

const (
	RoleAdmin   = "admin"
	RoleManager = "manager"
	RoleCashier = "cashier"
)

func IsValidRole(role string) bool {
	switch role {
	case RoleAdmin, RoleManager, RoleCashier:
		return true
	}
	return false
}

const (
	PaymentCash    = "cash"
	PaymentCard    = "card"
	PaymentEWallet = "ewallet"
)

func IsValidPaymentMethod(method string) bool {
	switch method {
	case PaymentCash, PaymentCard, PaymentEWallet:
		return true
	}
	return false
}

const (
	StockReasonSale       = "sale"
	StockReasonPurchase   = "purchase"
	StockReasonAdjustment = "adjustment"
)
