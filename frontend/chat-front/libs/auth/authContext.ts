import { createContext } from "react";
import { type UserManager } from "oidc-client-ts";

export interface AuthState {
  token: string | null;
  userId: string | null;
  userManager: UserManager | null;
  loading: boolean;
  setSession: (token: string, userId: string) => void;
  clearSession: () => void;
}

export const AuthContext = createContext<AuthState>({
  token: null,
  userId: null,
  userManager: null,
  loading: true,
  setSession: () => {},
  clearSession: () => {},
});
