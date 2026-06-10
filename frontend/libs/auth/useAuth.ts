import { useContext } from "react";
import { AuthContext } from "./authContext";

export function useAuth() {
  return useContext(AuthContext);
}
