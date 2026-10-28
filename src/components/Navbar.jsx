import { Link, useLocation } from "react-router-dom";

export default function Navbar({ activePage }) {
  const location = useLocation();

  // Navigation Links configuration
  const navLinks = [
    { name: "Home", path: "/", external: false },
    { name: "Instructions", path: "/instructions", external: false },
    { name: "Socials", path: "https://www.instagram.com/ccs_tiet/", external: true },
  ];

  return (
    <header className="w-full relative z-30 flex items-center justify-between px-6 sm:px-12 py-5 max-w-7xl mx-auto shrink-0">
      {/* --- LOGO & BRAND SECTION --- */}
      <Link to="/" className="flex items-center gap-3 group cursor-pointer">
        {/* TINKER: Change logo height using h-10, h-12, h-14, etc. */}
        <img
          src="/ccs-logo.png"
          alt="CCS Logo"
          className="h-10 sm:h-12 w-auto object-contain transition-transform duration-150 group-hover:scale-105"
        />
        {/* TINKER: Change brand text size using text-base, text-lg, text-xl, text-2xl */}
        <span className="font-display font-bold text-lg sm:text-xl text-[var(--ink)] tracking-tight">
          Creative Computing Society
        </span>
      </Link>

      {/* --- QUICK ACCESS LINKS RIBBON --- */}
      {/* TINKER: Change gap between links using gap-6, gap-8, gap-10 */}
      <nav className="flex items-center gap-6 sm:gap-8">
        {navLinks.map((link) => {
          const isActive = activePage
            ? link.name.toLowerCase() === activePage.toLowerCase()
            : location.pathname === link.path;

          {/* TINKER: Change link font size using text-sm, text-base, text-lg, text-xl */}
          const linkStyle = `text-sm sm:text-base font-medium transition-colors cursor-pointer ${
            isActive
              ? "text-[#3A94C2] font-semibold"
              : "text-gray-800 hover:text-[#3A94C2]"
          }`;

          if (link.external) {
            return (
              <a
                key={link.name}
                href={link.path}
                target="_blank"
                rel="noopener noreferrer"
                className={linkStyle}
              >
                {link.name}
              </a>
            );
          }

          return (
            <Link
              key={link.name}
              to={link.path}
              className={linkStyle}
            >
              {link.name}
            </Link>
          );
        })}
      </nav>
    </header>
  );
}
