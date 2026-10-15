package domain

const (
	RoleAdmin   = "admin"
	RoleManager = "manager"
	RoleCashier = "cashier"
)

// IsValidRole reports whether role is one of the known user roles.
//
// ENG: Valid roles are admin, manager and cashier.
// LAO: ກວດວ່າ role ເປັນ admin, manager ຫຼື cashier ຫຼືບໍ່.
//
// @param role - role name to check
// @returns true if the role is known
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

// IsValidPaymentMethod reports whether method is a supported payment method.
//
// ENG: Valid methods are cash, card and ewallet.
// LAO: ກວດວ່າວິທີຊຳລະເງິນເປັນ cash, card ຫຼື ewallet ຫຼືບໍ່.
//
// @param method - payment method to check
// @returns true if the method is supported
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
