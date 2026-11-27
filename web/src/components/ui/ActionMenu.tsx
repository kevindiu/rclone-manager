import { MoreHorizontal } from 'lucide-react';
import { type CSSProperties, type ReactNode, useEffect, useRef, useState } from 'react';

export interface ActionMenuItem {
  label: string;
  icon?: ReactNode;
  onClick: () => void;
  danger?: boolean;
  disabled?: boolean;
  hidden?: boolean;
}

export function ActionMenu({
  items,
  ariaLabel = 'More actions',
}: {
  items: ActionMenuItem[];
  ariaLabel?: string;
}) {
  const visibleItems = items.filter((item) => !item.hidden);
  const [open, setOpen] = useState(false);
  const [menuStyle, setMenuStyle] = useState<CSSProperties>({});
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) return;

    const updatePosition = () => {
      const btn = triggerRef.current;
      if (!btn) return;
      const rect = btn.getBoundingClientRect();
      const right = Math.max(8, window.innerWidth - rect.right);
      const openUp = window.innerHeight - rect.bottom < 220 && rect.top > 200;
      setMenuStyle(
        openUp
          ? { right, bottom: window.innerHeight - rect.top + 6 }
          : { right, top: rect.bottom + 6 }
      );
    };

    updatePosition();

    const handleMouseDown = (e: MouseEvent) => {
      const target = e.target as Node;
      if (triggerRef.current?.contains(target) || menuRef.current?.contains(target)) {
        return;
      }
      setOpen(false);
    };

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };

    const handleScrollOrResize = () => {
      setOpen(false);
    };

    document.addEventListener('mousedown', handleMouseDown);
    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('scroll', handleScrollOrResize, true);
    window.addEventListener('resize', handleScrollOrResize);
    return () => {
      document.removeEventListener('mousedown', handleMouseDown);
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('scroll', handleScrollOrResize, true);
      window.removeEventListener('resize', handleScrollOrResize);
    };
  }, [open]);

  if (visibleItems.length === 0) return null;

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        aria-label={ariaLabel}
        aria-haspopup="menu"
        aria-expanded={open}
        title={ariaLabel}
        onClick={() => setOpen((v) => !v)}
        className={`p-1.5 rounded-lg border transition-colors ${
          open
            ? 'bg-slate-700 border-slate-600 text-slate-100'
            : 'bg-slate-800 hover:bg-slate-700 border-transparent text-slate-300'
        }`}
      >
        <MoreHorizontal className="w-4 h-4" />
      </button>

      {open && (
        <div
          ref={menuRef}
          role="menu"
          style={menuStyle}
          className="fixed z-50 min-w-[176px] py-1 rounded-xl bg-slate-900 border border-slate-700 shadow-2xl"
        >
          {visibleItems.map((item) => (
            <button
              key={item.label}
              type="button"
              role="menuitem"
              disabled={item.disabled}
              onClick={() => {
                setOpen(false);
                item.onClick();
              }}
              className={`w-full px-3 py-2 text-left text-xs font-medium flex items-center gap-2 transition-colors disabled:opacity-40 ${
                item.danger
                  ? 'text-rose-300 hover:bg-rose-500/15'
                  : 'text-slate-200 hover:bg-slate-800'
              }`}
            >
              {item.icon && <span className="shrink-0">{item.icon}</span>}
              <span>{item.label}</span>
            </button>
          ))}
        </div>
      )}
    </>
  );
}
