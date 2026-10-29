package ozon

import "github.com/jackc/pgx/v5/pgtype"

func pgtypeTextNull() pgtype.Text {
	return pgtype.Text{
		String: "",
		Valid:  false,
	}
}

func pgtypeTimestamptzNull() pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Valid: false,
	}
}
