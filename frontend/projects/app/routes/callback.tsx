/* eslint-disable react-refresh/only-export-components */
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";
import { useAuth } from "@libs/auth/useAuth";

export const Route = createFileRoute("/callback")({
  component: Callback,
});

function Callback() {
  const navigate = useNavigate();
  const { userManager, setSession } = useAuth();

  useEffect(() => {
    if (!userManager) return;

    userManager
      .signinRedirectCallback()
      .then((user) =>
        fetch("/identity/v1/exchange", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ token: user.id_token }),
        }),
      )
      .then((r) => r.json())
      .then(({ access_token }: { access_token: string }) => {
        const [, payloadB64] = access_token.split(".");
        const payload = JSON.parse(
          atob(payloadB64.replace(/-/g, "+").replace(/_/g, "/")),
        );
        setSession(access_token, payload.sub as string);
        navigate({ to: "/", search: { chatId: undefined } });
      })
      .catch((err) => {
        console.error("Auth callback failed:", err);
        navigate({ to: "/", search: { chatId: undefined } });
      });
  }, [userManager, navigate, setSession]);

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        height: "100vh",
      }}
    >
      Logging in…
    </div>
  );
}
