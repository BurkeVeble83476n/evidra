import { BrowserRouter, Routes, Route } from "react-router";
import { AuthProvider } from "./context/AuthContext";
import { EvidenceModeProvider } from "./hooks/useEvidenceMode";
import { Layout } from "./components/Layout";
import { Landing } from "./pages/Landing";
import { Onboarding } from "./pages/Onboarding";
import { Dashboard } from "./pages/Dashboard";
import { BenchLeaderboard } from "./pages/bench/BenchLeaderboard";
import { BenchDashboard } from "./pages/bench/BenchDashboard";
import { BenchRuns } from "./pages/bench/BenchRuns";
import { BenchRunDetail } from "./pages/bench/BenchRunDetail";
import { Designer } from "./pages/Designer";
import { Evidence } from "./pages/Evidence";

export function App() {
  return (
    <AuthProvider>
      <EvidenceModeProvider>
        <BrowserRouter>
          <Layout>
            <Routes>
              <Route path="/" element={<Landing />} />
              <Route path="/onboarding" element={<Onboarding />} />
              <Route path="/dashboard" element={<Dashboard />} />
              <Route path="/evidence" element={<Evidence />} />
              <Route path="/bench" element={<BenchLeaderboard />} />
              <Route path="/bench/dashboard" element={<BenchDashboard />} />
              <Route path="/bench/runs" element={<BenchRuns />} />
              <Route path="/bench/runs/:id" element={<BenchRunDetail />} />
              <Route path="/designer" element={<Designer />} />
            </Routes>
          </Layout>
        </BrowserRouter>
      </EvidenceModeProvider>
    </AuthProvider>
  );
}
