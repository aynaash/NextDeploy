# Next.js Boilerplate with TypeScript, Tailwind, Drizzle, and BetterAuth

Here's a comprehensive template setup for your Next.js application with all the requested features. This will give you a solid foundation for rapid development.

## Features Included

- Next.js 14 (App Router)
- TypeScript
- Tailwind CSS
- Drizzle ORM with Postgres
- BetterAuth (NextAuth.js alternative)
- Google & GitHub authentication
- Landing page template
- Dashboard template
- Responsive design
- Dark mode support

## Setup Instructions

### 1. Create a new Next.js project

```bash
npx create-next-app@latest my-app --typescript --tailwind --eslint
cd my-app
```

### 2. Install required dependencies

```bash
npm install drizzle-orm postgres @auth/core @auth/drizzle-adapter @auth/core @auth/js @auth/prisma-adapter next-auth @types/bcryptjs bcryptjs zod @radix-ui/react-slot class-variance-authority clsx tailwind-merge lucide-react next-themes @types/node os

npm install -D drizzle-kit @types/bcryptjs tsx
```

### 3. File Structure

```
my-app/
├── app/
│   ├── (auth)/
│   │   ├── login/
│   │   │   └── page.tsx
│   │   └── register/
│   │       └── page.tsx
│   ├── (dashboard)/
│   │   ├── layout.tsx
│   │   ├── page.tsx
│   │   └── components/
│   │       ├── sidebar.tsx
│   │       └── header.tsx
│   ├── (marketing)/
│   │   ├── layout.tsx
│   │   └── page.tsx
│   ├── api/
│   │   └── auth/
│   │       └── [...nextauth]/
│   │           └── route.ts
│   └── layout.tsx
├── components/
│   ├── ui/ (shadcn-like components)
│   ├── auth/
│   └── marketing/
├── db/
│   ├── schema.ts
│   └── index.ts
├── lib/
│   ├── auth.ts
│   └── db.ts
├── styles/
│   └── globals.css
├── .env
├── drizzle.config.ts
└── package.json
```

### 4. Database Setup (Drizzle + Postgres)

`db/schema.ts`:
```typescript
import { pgTable, text, timestamp, uuid } from "drizzle-orm/pg-core";

export const users = pgTable("user", {
  id: uuid("id").primaryKey().defaultRandom(),
  name: text("name"),
  email: text("email").notNull().unique(),
  emailVerified: timestamp("emailVerified", { mode: "date" }),
  image: text("image"),
});

export const accounts = pgTable(
  "account",
  {
    userId: uuid("userId")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    type: text("type").notNull(),
    provider: text("provider").notNull(),
    providerAccountId: text("providerAccountId").notNull(),
    refresh_token: text("refresh_token"),
    access_token: text("access_token"),
    expires_at: timestamp("expires_at"),
    token_type: text("token_type"),
    scope: text("scope"),
    id_token: text("id_token"),
    session_state: text("session_state"),
  }
);
```

`db/index.ts`:
```typescript
import { drizzle } from "drizzle-orm/postgres-js";
import postgres from "postgres";
import * as schema from "./schema";

const connectionString = process.env.DATABASE_URL!;
const client = postgres(connectionString);
export const db = drizzle(client, { schema });
```

### 5. Authentication Setup

`lib/auth.ts`:
```typescript
import { DrizzleAdapter } from "@auth/drizzle-adapter";
import NextAuth from "next-auth";
import GoogleProvider from "next-auth/providers/google";
import GitHubProvider from "next-auth/providers/github";
import { db } from "./db";

export const {
  handlers: { GET, POST },
  auth,
  signIn,
  signOut,
} = NextAuth({
  adapter: DrizzleAdapter(db),
  providers: [
    GoogleProvider({
      clientId: process.env.GOOGLE_CLIENT_ID!,
      clientSecret: process.env.GOOGLE_CLIENT_SECRET!,
    }),
    GitHubProvider({
      clientId: process.env.GITHUB_CLIENT_ID!,
      clientSecret: process.env.GITHUB_CLIENT_SECRET!,
    }),
  ],
  callbacks: {
    async session({ session, user }) {
      if (session.user) {
        session.user.id = user.id;
      }
      return session;
    },
  },
});
```

### 6. Landing Page Template

`app/(marketing)/page.tsx`:
```typescript
import { Button } from "@/components/ui/button";
import Link from "next/link";

export default function Home() {
  return (
    <div className="min-h-screen bg-gradient-to-b from-background to-muted">
      <header className="container flex h-16 items-center justify-between">
        <Link href="/" className="text-xl font-bold">
          MyApp
        </Link>
        <nav className="flex items-center gap-4">
          <Button variant="ghost" asChild>
            <Link href="/login">Sign In</Link>
          </Button>
          <Button asChild>
            <Link href="/register">Get Started</Link>
          </Button>
        </nav>
      </header>

      <main className="container flex flex-col items-center justify-center py-32">
        <h1 className="text-5xl font-bold tracking-tight sm:text-6xl">
          Build something amazing
        </h1>
        <p className="mt-6 max-w-2xl text-lg text-muted-foreground">
          A starter template for your Next.js project with all the modern tools
          you need.
        </p>
        <div className="mt-10 flex items-center gap-4">
          <Button size="lg" asChild>
            <Link href="/register">Get Started</Link>
          </Button>
          <Button size="lg" variant="outline" asChild>
            <Link href="/login">Sign In</Link>
          </Button>
        </div>
      </main>
    </div>
  );
}
```

### 7. Dashboard Template

`app/(dashboard)/page.tsx`:
```typescript
import { auth } from "@/lib/auth";
import { redirect } from "next/navigation";

export default async function Dashboard() {
  const session = await auth();

  if (!session) {
    redirect("/login");
  }

  return (
    <div className="flex-1 space-y-4 p-8 pt-6">
      <div className="flex items-center justify-between space-y-2">
        <h2 className="text-3xl font-bold tracking-tight">Dashboard</h2>
      </div>
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {/* Dashboard cards would go here */}
      </div>
    </div>
  );
}
```

### 8. Environment Variables

`.env`:
```env
DATABASE_URL="postgres://user:password@localhost:5432/dbname"
NEXTAUTH_SECRET="your-secret-here"
NEXTAUTH_URL="http://localhost:3000"

GOOGLE_CLIENT_ID="your-google-client-id"
GOOGLE_CLIENT_SECRET="your-google-client-secret"

GITHUB_CLIENT_ID="your-github-client-id"
GITHUB_CLIENT_SECRET="your-github-client-secret"
```

### 9. Utility Components

Create a `components/ui` folder with reusable components (similar to shadcn/ui). Here's an example button component:

`components/ui/button.tsx`:
```typescript
import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:opacity-50 disabled:pointer-events-none ring-offset-background",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground hover:bg-primary/90",
        destructive:
          "bg-destructive text-destructive-foreground hover:bg-destructive/90",
        outline:
          "border border-input hover:bg-accent hover:text-accent-foreground",
        secondary:
          "bg-secondary text-secondary-foreground hover:bg-secondary/80",
        ghost: "hover:bg-accent hover:text-accent-foreground",
        link: "underline-offset-4 hover:underline text-primary",
      },
      size: {
        default: "h-10 py-2 px-4",
        sm: "h-9 px-3 rounded-md",
        lg: "h-11 px-8 rounded-md",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return (
      <Comp
        className={cn(buttonVariants({ variant, size, className }))}
        ref={ref}
        {...props}
      />
    );
  }
);
Button.displayName = "Button";

export { Button, buttonVariants };
```

## Next Steps

1. Set up your Postgres database and update the connection string
2. Configure OAuth apps with Google and GitHub
3. Run migrations with Drizzle:
   ```bash
   npx drizzle-kit generate:pg
   npx drizzle-kit push:pg
   ```
4. Start developing your app!

This template gives you:
- Authentication flows (login, register, social auth)
- Protected dashboard routes
- Modern UI components
- Type safety throughout
- Database access via Drizzle ORM

You can now focus on building your specific application features rather than setting up the foundation.
