const path = require('path');
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const CssMinimizerPlugin = require('css-minimizer-webpack-plugin');
const TerserPlugin = require('terser-webpack-plugin');

module.exports = {
    mode: 'production',
    entry: {
        main: path.resolve(__dirname, 'src/main.js'),
        cycling: path.resolve(__dirname, 'src/cycling.js'),
        'img-previewer': path.resolve(__dirname, 'src/img-previewer.js'),
    },
    output: {
        path: path.resolve(__dirname, 'dist'),
        filename: '[name].min.js',
        publicPath: '/'
    },
    stats: {
        entrypoints: false,
        children: false
    },
    module: {
        rules: [
            {
                test: /\.(scss|css)$/,
                use: [
                    MiniCssExtractPlugin.loader,
                    'css-loader',
                    'postcss-loader',
                    'sass-loader'
                ]
            },
            {
                test: /\.html$/,
                use: ['html-loader']
            }
        ],
    },
    resolve: {
        alias: {
            'iDisqus.css': 'disqus-php-api/dist/iDisqus.css',
        }
    },
    plugins: [
        new MiniCssExtractPlugin({
            filename: '[name].min.css'
        })
    ],
    optimization: {
        minimize: true,
        minimizer: [
            new TerserPlugin({
                parallel: true
            }),
            new CssMinimizerPlugin()
        ],
    }
};