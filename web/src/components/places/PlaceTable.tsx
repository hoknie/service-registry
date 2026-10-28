import Link from "next/link";
import type { ReactNode } from "react";

import { Table, Td, Th, Tr } from "../ui/Table";

export type PlaceColumn<R> = { key: string; title: string; className?: string; numeric?: boolean; cell: (row: R) => ReactNode };

type Props<R> = {
  rows: R[];
  rowKey: (row: R) => string;
  nameTitle: string;
  href: (row: R) => string;
  icon: (row: R) => ReactNode;
  name: (row: R) => string;
  badges?: (row: R) => ReactNode;
  columns: PlaceColumn<R>[];
};

export function PlaceTable<R>({ rows, rowKey, nameTitle, href, icon, name, badges, columns }: Props<R>) {
  return (
    <Table>
      <thead>
        <tr>
          <Th>{nameTitle}</Th>
          {columns.map((c) => (
            <Th key={c.key} numeric={c.numeric} className={c.className}>
              {c.title}
            </Th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <Tr key={rowKey(row)}>
            <Td>
              <Link href={href(row)} className="flex items-center gap-2.5 font-medium text-ink hover:underline">
                {icon(row)}
                {name(row)}
              </Link>
              {badges?.(row)}
            </Td>
            {columns.map((c) => (
              <Td key={c.key} numeric={c.numeric} className={c.className}>
                {c.cell(row)}
              </Td>
            ))}
          </Tr>
        ))}
      </tbody>
    </Table>
  );
}
