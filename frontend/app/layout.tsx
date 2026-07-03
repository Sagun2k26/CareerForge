import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import { AuthProvider } from "@/components/AuthProvider";
import Nav from "@/components/Nav";

const inter = Inter({ subsets: ["latin"], variable: "--font-inter" });

export const metadata: Metadata = {
  title: "CareerForge — AI Career Growth Platform",
  description:
    "Upload your resume, pick a target role, and get an AI-scored skill gap, a personalized roadmap, scored mock interviews, and role-specific prep.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={inter.variable}>
      <body className="font-sans">
        <AuthProvider>
          <div className="flex min-h-screen">
            <Nav />
            <main className="min-w-0 flex-1">{children}</main>
          </div>
        </AuthProvider>
      </body>
    </html>
  );
}
