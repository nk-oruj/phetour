<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    <xsl:output method="xml" omit-xml-declaration="yes"/>
    <xsl:strip-space elements="*"/>

    <xsl:template match="/library-update">
        <rss-content>
            <title>Library update</title>
            <description>
                <xsl:apply-templates select="posts"/>
                <xsl:apply-templates select="catalogs"/>
            </description>
        </rss-content>
    </xsl:template>

    <xsl:template match="posts">
        <xsl:apply-templates select="added|revised|removed"/>
    </xsl:template>

    <xsl:template match="catalogs">
        <xsl:apply-templates select="added|revised|removed"/>
    </xsl:template>

    <xsl:template match="posts/added">
        <p><strong>Added posts</strong></p>
        <ul><xsl:apply-templates select="post" mode="page"/></ul>
    </xsl:template>

    <xsl:template match="posts/revised">
        <p><strong>Revised posts</strong></p>
        <ul><xsl:apply-templates select="post" mode="page"/></ul>
    </xsl:template>

    <xsl:template match="posts/removed">
        <p><strong>Removed posts</strong></p>
        <ul><xsl:apply-templates select="post" mode="page"/></ul>
    </xsl:template>

    <xsl:template match="catalogs/added">
        <p><strong>Added catalogs</strong></p>
        <ul><xsl:apply-templates select="tag" mode="page"/></ul>
    </xsl:template>

    <xsl:template match="catalogs/revised">
        <p><strong>Updated catalogs</strong></p>
        <ul><xsl:apply-templates select="tag" mode="catalog"/></ul>
    </xsl:template>

    <xsl:template match="catalogs/removed">
        <p><strong>Removed catalogs</strong></p>
        <ul><xsl:apply-templates select="tag" mode="page"/></ul>
    </xsl:template>

    <xsl:template match="post|tag" mode="page">
        <li><a href="{concat(/library-update/@site-url, '/', @id, '/')}">[<xsl:value-of select="@id"/>] - <xsl:value-of select="@title"/></a></li>
    </xsl:template>

    <xsl:template match="tag" mode="catalog">
        <li>
            <a href="{concat(/library-update/@site-url, '/', @id, '/')}">[<xsl:value-of select="@id"/>] - <xsl:value-of select="@title"/></a>
            <xsl:if test="added-member or removed-member">
                <ul>
                    <xsl:for-each select="added-member">
                        <li>Added: <a href="{concat(/library-update/@site-url, '/', @id, '/')}"><xsl:value-of select="@title"/></a></li>
                    </xsl:for-each>
                    <xsl:for-each select="removed-member">
                        <li>Removed: [<xsl:value-of select="@id"/>] - <xsl:value-of select="@title"/></li>
                    </xsl:for-each>
                </ul>
            </xsl:if>
        </li>
    </xsl:template>
</xsl:stylesheet>
