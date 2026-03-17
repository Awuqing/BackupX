export interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

export interface ApiErrorPayload {
  code?: number;
  message?: string;
}
