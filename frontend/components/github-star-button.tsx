import { Github, Star } from 'lucide-react'

async function getStarCount(): Promise<number | null> {
    try {
        const res = await fetch('https://api.github.com/repos/aynaash/NextDeploy', {
            next: { revalidate: 3600 }, // cache for 1 hour
        })
        if (!res.ok) return null
        const data = await res.json()
        return data.stargazers_count ?? null
    } catch {
        return null
    }
}

export default async function GitHubStarButton() {
    const stars = await getStarCount()

    return (
        <a
            href="https://github.com/aynaash/NextDeploy"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 hover:border-slate-600 text-white text-sm font-medium transition-all duration-200 group"
        >
            <Github className="w-4 h-4 text-gray-400 group-hover:text-white transition-colors" />
            <span>Star on GitHub</span>
            {stars !== null && (
                <>
                    <span className="w-px h-4 bg-slate-600" />
                    <span className="flex items-center gap-1 text-yellow-400 font-semibold">
                        <Star className="w-3.5 h-3.5 fill-yellow-400" />
                        {stars >= 1000 ? `${(stars / 1000).toFixed(1)}k` : stars}
                    </span>
                </>
            )}
        </a>
    )
}
