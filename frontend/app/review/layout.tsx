import type { Metadata } from "next";

export const metadata: Metadata = {
  robots: {
    index: false,
    follow: false,
    nocache: true,
    noarchive: true,
  },
};

export default function ReviewLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <meta name="referrer" content="no-referrer" />
      {children}
    </>
  );
}
