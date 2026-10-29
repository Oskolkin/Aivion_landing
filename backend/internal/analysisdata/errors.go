package analysisdata

import "errors"

// ErrInvalidSellerAccountID is returned when seller_account_id is not positive.
var ErrInvalidSellerAccountID = errors.New("seller_account_id must be > 0")
