export type UserDto = {
  id: number;
  email: string;
  status: string;
};

export type SellerAccountDto = {
  id: number;
  name: string;
  status: string;
};

export type AuthResponse = {
  user: UserDto;
  seller_account: SellerAccountDto | null;
  is_admin: boolean;
  is_demo_seller?: boolean;
  is_review_mode?: boolean;
  is_live_review?: boolean;
  read_only?: boolean;
};

export type PublicAuthConfig = {
  public_registration: boolean;
  public_demo_mode: boolean;
};

export type RegisterRequest = {
  email: string;
  password: string;
  password_confirm: string;
};

export type LoginRequest = {
  email: string;
  password: string;
};
