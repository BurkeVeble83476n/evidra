import { BrowserRouter, Routes, Route } from "react-router";
import { AuthProvider } from "./context/AuthContext";
import { Layout } from "./components/Layout";
import { Landing } from "./pages/Landing";
import { Onboarding } from "./pages/Onboarding";
import { Dashboard } from "./pages/Dashboard";

export function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Layout>
          <Routes>
            <Route path="/" element={<Landing />} />
            <Route path="/onboarding" element={<Onboarding />} />
            <Route path="/dashboard" element={<Dashboard />} />
          </Routes>
        </Layout>
      </BrowserRouter>
    </AuthProvider>
  );
}
