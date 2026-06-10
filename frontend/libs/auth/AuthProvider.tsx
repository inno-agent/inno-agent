import { useEffect, useRef, useState } from "react";
import { type UserManager } from "oidc-client-ts";
import { createUserManager } from "./authClient";
import { AuthContext, type AuthState } from "./authContext";

export function AuthProvider({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const [state, setState] = useState<AuthState>(() => {
    const isCallback = window.location.pathname === "/callback";
    const token = localStorage.getItem("aicore_token");
    const userId = localStorage.getItem("aicore_user_id");
    return {
      token: token && !isCallback ? token : null,
      userId: userId && !isCallback ? userId : null,
      userManager: null,
      loading: true,
    };
  });
  const initDone = useRef(false);

  useEffect(() => {
    if (initDone.current) return;
    initDone.current = true;

    const isCallback = window.location.pathname === "/callback";

    createUserManager().then((um: UserManager) => {
      setState((prev) => ({
        ...prev,
        userManager: um,
        loading: isCallback ? true : !prev.token,
      }));
      if (!isCallback && !state.token) {
        um.signinRedirect();
      }
    });
  }, [state.token]);

  return <AuthContext.Provider value={state}>{children}</AuthContext.Provider>;
}
