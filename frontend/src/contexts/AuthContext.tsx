"use client";

import { createContext, useContext, useState, useEffect, ReactNode } from "react";
import { useRouter } from "next/navigation";

interface AuthUser {
  username: string;
  token: string;
}

interface AuthContextType {
  user: AuthUser | null;
  login: (username: string, password: string) => Promise<{ success: boolean; error?: string }>;
  logout: () => void;
  isLoading: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const router = useRouter();

  useEffect(() => {
    // Check for existing token on mount
    const token = localStorage.getItem("ojs_token");
    const username = localStorage.getItem("ojs_username");
    if (token && username) {
      setUser({ token, username });
    }
    setIsLoading(false);
  }, []);

  const login = async (username: string, password: string) => {
    try {
      const res = await fetch("http://localhost:8080/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });

      const data = await res.json();

      if (data.success && data.data) {
        const { token, username } = data.data;
        localStorage.setItem("ojs_token", token);
        localStorage.setItem("ojs_username", username);
        setUser({ token, username });
        return { success: true };
      } else {
        return { success: false, error: data.error || "Login gagal" };
      }
    } catch (err) {
      return { success: false, error: "Gagal terhubung ke server" };
    }
  };

  const logout = () => {
    localStorage.removeItem("ojs_token");
    localStorage.removeItem("ojs_username");
    setUser(null);
    router.push("/login");
  };

  return (
    <AuthContext.Provider value={{ user, login, logout, isLoading }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}

// Helper function to get auth headers
export function getAuthHeaders(): HeadersInit {
  const token = typeof window !== "undefined" ? localStorage.getItem("ojs_token") : null;
  if (token) {
    return {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    };
  }
  return { "Content-Type": "application/json" };
}
