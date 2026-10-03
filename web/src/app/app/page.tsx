import { Navigate, useLocation } from "react-router-dom";

// The search survives the redirect: /app?agent_session=… opens the assistant.
export default function AppDefault() {
  const { search } = useLocation();
  return <Navigate to={{ pathname: "/app/emails", search }} replace />;
}
