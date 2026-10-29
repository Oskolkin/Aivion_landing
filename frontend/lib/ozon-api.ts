import { apiGet, apiPost, apiPut } from "@/lib/api";

/** User-facing Ozon connection state (no secrets). */
export type OzonUserConnectionDto = {
  connection_ready: boolean;
  authentication_mode?: "PRIVATE_OAUTH" | "LEGACY" | string;
  status: "not_connected" | "connected" | "connection_error" | "disconnected_retention" | string;
  missing: string[];
  errors: Array<{
    source: "seller_api" | "performance_api" | string;
    code: string;
    message: string;
  }>;
  seller_client_id_masked?: string;
  seller_credentials_set: boolean;
  performance_secret_set: boolean;
  retention_delete_after?: string | null;
};

export type PutOzonUserConnectionRequest = {
  seller_client_id: string;
  seller_api_key: string;
  performance_client_id: string;
  performance_client_secret: string;
};

export type PutOzonUserConnectionResponse = {
  connection: OzonUserConnectionDto;
  message?: string;
  errors?: OzonUserConnectionDto["errors"];
  connection_ready?: boolean;
};

export async function getOzonUserConnection(): Promise<OzonUserConnectionDto> {
  return apiGet<OzonUserConnectionDto>("/api/v1/integrations/ozon/connection");
}

export async function putOzonUserConnection(
  payload: PutOzonUserConnectionRequest
): Promise<PutOzonUserConnectionResponse> {
  return apiPut<PutOzonUserConnectionResponse>("/api/v1/integrations/ozon/connection", payload);
}

export async function disconnectOzonCabinet(confirmation: string): Promise<{ connection: OzonUserConnectionDto }> {
  return apiPost<{ connection: OzonUserConnectionDto }>("/api/v1/integrations/ozon/disconnect", {
    confirmation,
  });
}
