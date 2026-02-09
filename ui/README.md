# Lute UI - VM Management Platform

A modern React application for managing virtual machines, built with Vite, TypeScript, and Firebase Authentication.

## Features

- 🔐 Firebase Authentication with Google Sign-in/Sign-up
- 📊 Dashboard with VM statistics
- 🖥️ User Machines page for managing personal VMs
- 🌐 Public Machines page for browsing shared VMs
- 🎨 Modern UI with Tailwind CSS
- ⚡ Fast development with Vite

## Getting Started

### Prerequisites

- Node.js 18+ and npm/yarn/pnpm

### Installation

1. Install dependencies:
```bash
npm install
```

2. Set up Firebase:
   - **See [FIREBASE_SETUP.md](./FIREBASE_SETUP.md) for detailed step-by-step instructions**
   - Quick steps:
     1. Create a Firebase project at [Firebase Console](https://console.firebase.google.com/)
     2. Enable Google Authentication
     3. Add a Web app and copy the configuration
     4. Create a `.env` file in the `ui` directory with your Firebase credentials

   For complete instructions, see [FIREBASE_SETUP.md](./FIREBASE_SETUP.md)

### Development

Start the development server:

```bash
npm run dev
```

The app will be available at `http://localhost:5173`

### Build

Build for production:

```bash
npm run build
```

The production build will be in the `dist` directory.

### Preview

Preview the production build:

```bash
npm run preview
```

## Project Structure

```
ui/
├── src/
│   ├── components/       # Reusable React components
│   │   ├── Layout.tsx
│   │   └── ProtectedRoute.tsx
│   ├── contexts/         # React contexts
│   │   └── AuthContext.tsx
│   ├── pages/           # Page components
│   │   ├── Dashboard.tsx
│   │   ├── Login.tsx
│   │   ├── UserMachines.tsx
│   │   └── PublicMachines.tsx
│   ├── services/         # API and service functions
│   │   └── authService.ts
│   ├── config/          # Configuration files
│   │   └── firebase.ts
│   ├── types/           # TypeScript type definitions
│   │   └── index.ts
│   ├── App.tsx          # Main App component
│   ├── main.tsx         # Application entry point
│   └── index.css        # Global styles
├── public/              # Static assets
├── index.html           # HTML template
├── vite.config.ts       # Vite configuration
├── tsconfig.json        # TypeScript configuration
└── package.json         # Dependencies and scripts
```

## Technologies

- **React 18** - UI library
- **TypeScript** - Type safety
- **Vite** - Build tool and dev server
- **React Router** - Client-side routing
- **Firebase Auth** - Authentication
- **Tailwind CSS** - Styling

## Next Steps

- Connect to your backend API for VM management
- Implement actual VM CRUD operations
- Add VM creation/editing forms
- Implement real-time updates
- Add error handling and loading states
- Set up state management (Redux/Zustand) if needed

