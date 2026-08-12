import AppShell from "@/components/AppShell";
import { DemoStoreProvider } from "@/store/DemoStore";
import CreatorPage from "@/pages/CreatorPage";
import ExplorePage from "@/pages/ExplorePage";
import HomePage from "@/pages/HomePage";
import ProfilePage from "@/pages/ProfilePage";
import SharePage from "@/pages/SharePage";
import TrustPage from "@/pages/TrustPage";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";

export default function App() {
  return (
    <BrowserRouter>
      <DemoStoreProvider>
        <Routes>
          <Route element={<AppShell />}>
            <Route index element={<HomePage />} />
            <Route path="explore" element={<ExplorePage />} />
            <Route path="creator" element={<CreatorPage />} />
            <Route path="trust" element={<TrustPage />} />
            <Route path="u/:handle" element={<ProfilePage />} />
            <Route path="share/:kind/:id" element={<SharePage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </DemoStoreProvider>
    </BrowserRouter>
  );
}
